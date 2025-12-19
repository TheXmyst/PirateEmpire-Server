package engine

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/TheXmyst/Sea-Dogs/server/internal/domain"
	"github.com/TheXmyst/Sea-Dogs/server/internal/economy"
	"github.com/TheXmyst/Sea-Dogs/server/internal/logger"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// ProcessPvpTravelFleets handles fleets traveling to/from PvP attacks
func ProcessPvpTravelFleets(db *gorm.DB) {
	var fleets []domain.Fleet

	// Get all fleets in PvP travel states
	if err := db.Preload("Ships").Where("state IN ?", []string{"Traveling_To_Attack", "Returning_From_Attack"}).Find(&fleets).Error; err != nil {
		logger.Warn("[PVP_TRAVEL] Error loading fleets", "error", err)
		return
	}

	for _, fleet := range fleets {
		if fleet.State == "Traveling_To_Attack" {
			handleTravelingToAttack(db, &fleet)
		} else if fleet.State == "Returning_From_Attack" {
			handleReturningFromAttack(db, &fleet)
		}
	}
}

// handleTravelingToAttack checks if fleet arrived at target and triggers combat
func handleTravelingToAttack(db *gorm.DB, fleet *domain.Fleet) {
	if fleet.TargetX == nil || fleet.TargetY == nil || fleet.TargetIslandID == nil {
		logger.Warn("[PVP_TRAVEL] Fleet has invalid target, resetting to Idle", "fleet_id", fleet.ID)
		fleet.State = "Idle"
		db.Save(fleet)
		return
	}

	// Get fleet's current position (use first ship's position)
	if len(fleet.Ships) == 0 {
		logger.Warn("[PVP_TRAVEL] Fleet has no ships, resetting to Idle", "fleet_id", fleet.ID)
		fleet.State = "Idle"
		db.Save(fleet)
		return
	}

	// Calculate distance to target
	ship := fleet.Ships[0]
	distance := economy.CalculateDistance(int(ship.X), int(ship.Y), *fleet.TargetX, *fleet.TargetY)

	// Move ships toward target if not arrived
	if distance > 5 {
		// Move all ships in fleet toward target
		// Speed: 50 units/minute = 0.833 units/second
		// Game loop runs at 10Hz (0.1s), so movement per tick = 0.0833 units
		speed := 0.0833

		for i := range fleet.Ships {
			s := &fleet.Ships[i]

			// Calculate direction vector
			dx := float64(*fleet.TargetX) - s.X
			dy := float64(*fleet.TargetY) - s.Y
			dist := economy.CalculateDistance(int(s.X), int(s.Y), *fleet.TargetX, *fleet.TargetY)

			if dist > 0 {
				// Normalize and apply speed
				s.X += (dx / dist) * speed
				s.Y += (dy / dist) * speed
				db.Save(s)
			}
		}

		// Continue moving (no log spam)
		return
	}

	// Check if arrived (distance < 5 units)
	if distance < 5 {
		logger.Info("[PVP] Fleet arrived at target, triggering combat", "fleet_id", fleet.ID)

		// Trigger combat
		combatResult, loot, err := executePvpCombat(db, fleet, *fleet.TargetIslandID)
		if err != nil {
			logger.Error("[PVP_TRAVEL] Combat error", "error", err)
			fleet.State = "Idle"
			db.Save(fleet)
			return
		}

		// Store loot in fleet
		if combatResult.Winner == "fleet_a" && len(loot) > 0 {
			// 1. Keep AttackLoot for "Battle Report" UI
			fleet.AttackLoot = loot
			lootJSON, _ := json.Marshal(loot)
			fleet.AttackLootJSON = lootJSON

			// 2. Add to Cargo for physical transport
			if fleet.Cargo == nil {
				fleet.Cargo = make(map[domain.ResourceType]float64)
			}
			for res, amount := range loot {
				fleet.Cargo[res] += amount
			}
		}

		// Set returning state
		fleet.State = "Returning_From_Attack"
		fleet.TargetIslandID = nil

		// Set target to home island
		var homeIsland domain.Island
		if err := db.Where("id = ?", fleet.IslandID).First(&homeIsland).Error; err == nil {
			fleet.TargetX = &homeIsland.X
			fleet.TargetY = &homeIsland.Y
		}

		db.Save(fleet)
		logger.Info("[PVP_TRAVEL] Fleet returning home with loot", "fleet_id", fleet.ID)
	} else {
		// Continue moving (no log spam)
	}
}

// handleReturningFromAttack checks if fleet arrived home and deposits loot
func handleReturningFromAttack(db *gorm.DB, fleet *domain.Fleet) {
	if fleet.TargetX == nil || fleet.TargetY == nil {
		logger.Warn("[PVP_TRAVEL] Fleet has invalid return target, resetting to Idle", "fleet_id", fleet.ID)
		fleet.State = "Idle"
		db.Save(fleet)
		return
	}

	// Get fleet's current position
	if len(fleet.Ships) == 0 {
		logger.Warn("[PVP_TRAVEL] Fleet has no ships, resetting to Idle", "fleet_id", fleet.ID)
		fleet.State = "Idle"
		db.Save(fleet)
		return
	}

	// Calculate distance to home
	ship := fleet.Ships[0]
	distance := economy.CalculateDistance(int(ship.X), int(ship.Y), *fleet.TargetX, *fleet.TargetY)

	// Move ships toward home if not arrived
	if distance > 5 {
		// Move all ships in fleet toward home
		speed := 0.0833 // Same speed as outbound journey

		for i := range fleet.Ships {
			s := &fleet.Ships[i]

			// Calculate direction vector
			dx := float64(*fleet.TargetX) - s.X
			dy := float64(*fleet.TargetY) - s.Y
			dist := economy.CalculateDistance(int(s.X), int(s.Y), *fleet.TargetX, *fleet.TargetY)

			if dist > 0 {
				// Normalize and apply speed
				s.X += (dx / dist) * speed
				s.Y += (dy / dist) * speed
				db.Save(s)
			}
		}

		// Continue moving (no log spam)
		return
	}

	// Check if arrived home (distance < 5 units)
	if distance < 5 {
		logger.Info("[PVP_TRAVEL] Fleet arrived home, depositing loot", "fleet_id", fleet.ID)

		// Deposit loot (From Cargo now, covering both Loot and any other stock)
		depositedLog := ""
		if fleet.Cargo != nil && len(fleet.Cargo) > 0 {
			var island domain.Island
			if err := db.Where("id = ?", fleet.IslandID).First(&island).Error; err == nil {
				// Deserialize resources if needed
				if island.Resources == nil {
					island.Resources = make(map[domain.ResourceType]float64)
				}

				// Add all Cargo to island
				for resType, amount := range fleet.Cargo {
					if amount > 0 {
						island.Resources[resType] += amount
						depositedLog += fmt.Sprintf("%.0f %s, ", amount, resType)
					}
				}

				// Serialize resources
				resJSON, _ := json.Marshal(island.Resources)
				island.ResourcesJSON = resJSON

				db.Save(&island)
			}
		}

		// Fallback: If Cargo was empty but AttackLoot existed (migration case?), deposit AttackLoot
		// This handles in-flight fleets during update
		if (fleet.Cargo == nil || len(fleet.Cargo) == 0) && len(fleet.AttackLoot) > 0 {
			var island domain.Island
			if err := db.Where("id = ?", fleet.IslandID).First(&island).Error; err == nil {
				// Deserialize resources if needed
				if island.Resources == nil {
					island.Resources = make(map[domain.ResourceType]float64)
				}
				for resType, amount := range fleet.AttackLoot {
					island.Resources[resType] += amount
					depositedLog += fmt.Sprintf("%.0f %s (legacy), ", amount, resType)
				}
				resJSON, _ := json.Marshal(island.Resources)
				island.ResourcesJSON = resJSON
				db.Save(&island)
			}
		}

		// Reset fleet
		fleet.State = "Idle"
		fleet.TargetX = nil
		fleet.TargetY = nil
		fleet.AttackLoot = nil
		fleet.AttackLootJSON = nil
		fleet.Cargo = nil // Clear cargo after deposit

		db.Save(fleet)
		logger.Info("[PVP_TRAVEL] Fleet reset to Idle", "fleet_id", fleet.ID)
	} else {
		// Continue moving (no log spam)
	}
}

// executePvpCombat triggers the PvP combat logic (simplified version)
// Returns combat result and loot
func executePvpCombat(db *gorm.DB, attackerFleet *domain.Fleet, targetIslandID uuid.UUID) (*economy.CombatResult, map[domain.ResourceType]float64, error) {
	// Load target island with buildings
	var targetIsland domain.Island
	if err := db.Preload("Buildings").Preload("Player").First(&targetIsland, "id = ?", targetIslandID).Error; err != nil {
		return nil, nil, fmt.Errorf("target island not found: %v", err)
	}

	// Load attacker island
	var attackerIsland domain.Island
	if err := db.First(&attackerIsland, "id = ?", attackerFleet.IslandID).Error; err != nil {
		return nil, nil, fmt.Errorf("attacker island not found: %v", err)
	}

	// Get defender fleet (use active fleet or militia)
	var defenderFleet domain.Fleet
	hasDefenderFleet := false

	if targetIsland.ActiveFleetID != nil {
		if err := db.Preload("Ships").Where("id = ?", *targetIsland.ActiveFleetID).First(&defenderFleet).Error; err == nil {
			hasDefenderFleet = true
		}
	}

	// If no active fleet, create militia fleet (simplified)
	if !hasDefenderFleet {
		defenderFleet = domain.Fleet{
			ID:       uuid.New(),
			IslandID: targetIsland.ID,
			Name:     "Militia",
			State:    "Idle",
			Ships:    []domain.Ship{}, // Empty fleet = militia uses island garrison
		}
	}

	// Get captains
	var attackerCaptain, defenderCaptain *domain.Captain
	// TODO: Load actual captains if needed

	// Compute engagement
	engResult := economy.ComputeEngagementMorale(*attackerFleet, defenderFleet, attackerCaptain, defenderCaptain)

	// Execute combat
	combatRes, err := economy.ExecuteNavalCombat(attackerFleet, &defenderFleet, attackerCaptain, defenderCaptain, engResult, time.Now().UnixNano())
	if err != nil {
		return nil, nil, err
	}

	// Calculate loot if attacker won
	loot := make(map[domain.ResourceType]float64)
	if combatRes.Winner == "fleet_a" {
		defenderTH := economy.GetBuildingLevel(&targetIsland, "Hôtel de Ville")
		safeAmount := float64(defenderTH * 1000)

		// Deserialize target island resources
		if len(targetIsland.ResourcesJSON) > 0 {
			json.Unmarshal(targetIsland.ResourcesJSON, &targetIsland.Resources)
		}

		// Steal 50% of vulnerable resources
		for res, amount := range targetIsland.Resources {
			available := amount - safeAmount
			if available > 0 {
				stolen := available * 0.50
				loot[res] = stolen

				// Deduct from target island
				targetIsland.Resources[res] -= stolen
			}
		}

		// Apply peace shield to defender
		shieldEnd := time.Now().Add(4 * time.Hour)
		targetIsland.ProtectedUntil = &shieldEnd

		// Save target island
		resJSON, _ := json.Marshal(targetIsland.Resources)
		targetIsland.ResourcesJSON = resJSON
		db.Save(&targetIsland)
	}

	// Destroy ships (attacker only in this simplified version)
	for _, id := range combatRes.ShipsDestroyedA {
		economy.DestroyShipHard(db, id)
	}

	return &combatRes, loot, nil
}
