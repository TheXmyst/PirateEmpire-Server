package engine

import (
	"encoding/json"
	"fmt"
	"math"
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
	states := []string{
		string(domain.FleetStateTravelingToAttack),
		string(domain.FleetStateReturningFromAttack),
		string(domain.FleetStateChasingPvE),
	}
	if err := db.Preload("Ships").Preload("Island.Player.Captains").Where("state IN ?", states).Find(&fleets).Error; err != nil {
		logger.Warn("[PVP_TRAVEL] Error loading fleets", "error", err)
		return
	}

	for _, fleet := range fleets {
		if fleet.State == domain.FleetStateTravelingToAttack {
			handleTravelingToAttack(db, &fleet)
		} else if fleet.State == domain.FleetStateReturningFromAttack {
			handleReturningFromAttack(db, &fleet)
		} else if fleet.State == domain.FleetStateChasingPvE {
			handleChasingPvE(db, &fleet)
		}
	}
}

// handleChasingPvE handles tracking of moving PvE targets
func handleChasingPvE(db *gorm.DB, fleet *domain.Fleet) {
	if fleet.TargetPveID == nil {
		fmt.Printf("[PVE_TRACK] abort fleet=%s reason=no_target_id\n", fleet.Name)
		returnHome(db, fleet)
		return
	}

	// Load PvE Target
	target := economy.GetPveTargetByUUID(fleet.IslandID, *fleet.TargetPveID) // Use Fleet IslandID as PlayerID proxy if needed?
	// Wait, GetPveTargetByUUID needs PlayerID. Fleet.IslandID is Island UUID.
	// We loaded fleet.Island.Player.Captains but not fleet.Island.Player?
	// Actually ProcessPvpTravelFleets preloads Island.Player.Captains, so Island.PlayerID is available.
	// We need to pass the PlayerID to GetPveTargetByUUID.
	// The fleets query preloads Island.Player...

	// Refetch PlayerID if missing (safety)
	var playerID uuid.UUID
	if fleet.Island.PlayerID != uuid.Nil {
		playerID = fleet.Island.PlayerID
	} else {
		// Fallback lookup
		var island domain.Island
		if err := db.Select("player_id").First(&island, "id = ?", fleet.IslandID).Error; err == nil {
			playerID = island.PlayerID
		}
	}

	// Try to get target
	target = economy.GetPveTargetByUUID(playerID, *fleet.TargetPveID)

	if target == nil {
		fmt.Printf("[PVE_TRACK] abort fleet=%s reason=target_lost_or_expired\n", fleet.Name)
		returnHome(db, fleet)
		return
	}

	// Update Fleet Target Coords to match Moving Target (Tracking)
	tx := int(target.RealX)
	ty := int(target.RealY)
	fleet.TargetX = &tx
	fleet.TargetY = &ty

	// Get Fleet Position
	if len(fleet.Ships) == 0 {
		returnHome(db, fleet)
		return
	}
	ship := fleet.Ships[0]

	// Calculate Distance
	dx := target.RealX - ship.X
	dy := target.RealY - ship.Y
	distance := math.Sqrt(dx*dx + dy*dy)

	// Check Engage Radius (5.0)
	if distance < 5.0 {
		fmt.Printf("[PVE_TRACK] engage fleet=%s target=%s dist=%.2f\n", fleet.Name, target.Name, distance)

		// Trigger Combat
		combatResult, loot, err := executePveCombat(db, fleet, target)
		if err != nil {
			logger.Error("[PVE_TRACK] Combat error", "error", err)
			returnHome(db, fleet)
			return
		}

		// Handle Loot
		if combatResult.Winner == "fleet_a" && len(loot) > 0 {
			fleet.AttackLoot = loot
			lootJSON, _ := json.Marshal(loot)
			fleet.AttackLootJSON = lootJSON

			if fleet.Cargo == nil {
				fleet.Cargo = make(map[domain.ResourceType]float64)
			}
			for res, amount := range loot {
				fleet.Cargo[res] += amount
			}
		}

		// Consume Target (Remove from cache)
		economy.ConsumePveTarget(playerID, target.ID.String())

		// Return Home
		returnHome(db, fleet)
		return
	}

	// Move Fleet
	// Log (Throttled)
	if time.Now().UnixNano()/int64(time.Millisecond)%5000 < 100 { // ~5s interval
		fmt.Printf("[PVE_TRACK] chasing fleet=%s target=%s dist=%.2f\n", fleet.Name, target.Name, distance)
	}

	// SSOT Speed
	speedSec := economy.ComputeTravelSpeed(fleet, false) // isNPC=false
	speed := speedSec * 0.1

	// Rum Consumption & Penalty
	hasRum := fleet.Cargo != nil && fleet.Cargo[domain.Rum] > 0
	if !hasRum {
		speed *= 0.8
	}

	consPerSec := economy.ComputeRumConsumption(len(fleet.Ships), false)
	consPerTick := consPerSec * 0.1

	if fleet.Cargo == nil {
		fleet.Cargo = make(map[domain.ResourceType]float64)
	}
	if fleet.Cargo[domain.Rum] > 0 {
		fleet.Cargo[domain.Rum] -= consPerTick
		if fleet.Cargo[domain.Rum] < 0 {
			fleet.Cargo[domain.Rum] = 0
		}
	}

	// Move Ships
	if distance > 0 {
		for i := range fleet.Ships {
			s := &fleet.Ships[i]
			s.X += (dx / distance) * speed
			s.Y += (dy / distance) * speed
			db.Save(s)
		}
	}

	db.Save(fleet)
}

func returnHome(db *gorm.DB, fleet *domain.Fleet) {
	fleet.State = domain.FleetStateReturningFromAttack
	fleet.TargetPveID = nil
	fleet.TargetIslandID = nil

	var homeIsland domain.Island
	if err := db.Where("id = ?", fleet.IslandID).First(&homeIsland).Error; err == nil {
		fleet.TargetX = &homeIsland.X
		fleet.TargetY = &homeIsland.Y
	}
	db.Save(fleet)
}

// executePveCombat triggers PvE combat
func executePveCombat(db *gorm.DB, attackerFleet *domain.Fleet, target *economy.PveTarget) (*economy.CombatResult, map[domain.ResourceType]float64, error) {
	// Generate NPC Fleet from Target
	npcFleet := economy.GenerateNpcFleet(target.Tier, target.ID.String())

	// Captain
	// We need to use the target.Captain
	// But ExecuteNavalCombat expects *domain.Captain
	// We can use target.Captain directly

	var attackerCaptain *domain.Captain
	// Load attacker captain
	// Simple lookup for now
	// TODO: Proper captain assignment lookup

	// Engage
	engResult := economy.ComputeEngagementMorale(*attackerFleet, npcFleet, attackerCaptain, &target.Captain)

	// Execute
	combatRes, err := economy.ExecuteNavalCombat(attackerFleet, &npcFleet, attackerCaptain, &target.Captain, engResult, time.Now().UnixNano())
	if err != nil {
		return nil, nil, err
	}

	// Loot Calculation (Simplified for PvE)
	loot := make(map[domain.ResourceType]float64)
	if combatRes.Winner == "fleet_a" {
		// Tier based loot
		baseLoot := float64(target.Tier * 1000)
		loot[domain.Gold] = baseLoot
		loot[domain.Wood] = baseLoot * 0.5
		if target.Tier >= 2 {
			loot[domain.Rum] = baseLoot * 0.2
		}
	}

	// Destroy Attacker Ships (if any)
	for _, id := range combatRes.ShipsDestroyedA {
		economy.DestroyShipHard(db, id)
	}

	return &combatRes, loot, nil
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
			State:    domain.FleetStateIdle,
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

// handleTravelingToAttack checks if fleet arrived at target and triggers combat
func handleTravelingToAttack(db *gorm.DB, fleet *domain.Fleet) {
	if fleet.TargetX == nil || fleet.TargetY == nil || fleet.TargetIslandID == nil {
		logger.Warn("[PVP_TRAVEL] Fleet has invalid target, resetting to Idle", "fleet_id", fleet.ID)
		fleet.State = domain.FleetStateIdle
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

	// Calculate distance to target (Float Precision!)
	ship := fleet.Ships[0]
	// distance := economy.CalculateDistance(int(ship.X), int(ship.Y), *fleet.TargetX, *fleet.TargetY)

	dx := float64(*fleet.TargetX) - ship.X
	dy := float64(*fleet.TargetY) - ship.Y
	distance := math.Sqrt(dx*dx + dy*dy)

	// Debug Log (Throttled)
	if time.Now().UnixNano()/int64(time.Millisecond)%5000 < 100 { // ~5s interval
		fmt.Printf("[PVP_TRAVEL] Fleet %s dist=%.4f target=(%d,%d) ship=(%.2f,%.2f)\n", fleet.Name, distance, *fleet.TargetX, *fleet.TargetY, ship.X, ship.Y)
	}

	// Move ships toward target if not arrived
	if distance > 5 {
		// Use Centralized Speed Calculation (SSOT)
		// Tick is 0.1s
		speedSec := economy.ComputeTravelSpeed(fleet, false) // isNPC=false (Player Fleet)
		speed := speedSec * 0.1                              // Speed per Tick

		// RUM MECHANIC
		hasRum := fleet.Cargo != nil && fleet.Cargo[domain.Rum] > 0
		if !hasRum {
			speed *= 0.8
		}

		// Consumption (SSOT w/ Multiplier)
		// Tick is 0.1s
		consPerSec := economy.ComputeRumConsumption(len(fleet.Ships), false)
		consPerTick := consPerSec * 0.1

		if fleet.Cargo == nil {
			fleet.Cargo = make(map[domain.ResourceType]float64)
		}
		if fleet.Cargo[domain.Rum] > 0 {
			fleet.Cargo[domain.Rum] -= consPerTick
			if fleet.Cargo[domain.Rum] < 0 {
				fleet.Cargo[domain.Rum] = 0
			}
		}

		// Log (Throttled ~1%) - Only log if moving
		if time.Now().UnixNano()/int64(time.Millisecond)%10000 < 100 {
			fmt.Printf("[NAV] fleet=%s speed=%.4f/sec (%.4f/tick)\n", fleet.Name, speedSec, speed)
			fmt.Printf("[RUM] fleet=%s cons=%.4f/sec (%.4f/tick) remaining=%.1f\n", fleet.Name, consPerSec, consPerTick, fleet.Cargo[domain.Rum])
		}

		for i := range fleet.Ships {
			s := &fleet.Ships[i]

			// Calculate direction vector (Using Float)
			dx := float64(*fleet.TargetX) - s.X
			dy := float64(*fleet.TargetY) - s.Y
			dist := math.Sqrt(dx*dx + dy*dy)

			if dist > 0 {
				// Normalize and apply speed
				s.X += (dx / dist) * speed
				s.Y += (dy / dist) * speed
				db.Save(s)
			}
		}

		// Persist consumption
		db.Save(fleet)

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
			fleet.State = domain.FleetStateIdle
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
		fleet.State = domain.FleetStateReturningFromAttack
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
		fleet.State = domain.FleetStateIdle
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

	// Calculate distance to home (Float Precision!)
	ship := fleet.Ships[0]
	// distance := economy.CalculateDistance(int(ship.X), int(ship.Y), *fleet.TargetX, *fleet.TargetY)

	dx := float64(*fleet.TargetX) - ship.X
	dy := float64(*fleet.TargetY) - ship.Y
	distance := math.Sqrt(dx*dx + dy*dy)

	// Debug Log (Throttled)
	if time.Now().UnixNano()/int64(time.Millisecond)%5000 < 100 {
		fmt.Printf("[PVP_TRAVEL] Fleet %s (returning) dist=%.4f\n", fleet.Name, distance)
	}

	// Move ships toward home if not arrived
	if distance > 5 {
		// Use Centralized Speed Calculation (SSOT)
		speedSec := economy.ComputeTravelSpeed(fleet, false) // isNPC=false
		speed := speedSec * 0.1                              // Speed per tick

		// RUM MECHANIC
		hasRum := fleet.Cargo != nil && fleet.Cargo[domain.Rum] > 0
		if !hasRum {
			speed *= 0.8
		}

		// Consumption (SSOT w/ Multiplier)
		// Tick is 0.1s
		consPerSec := economy.ComputeRumConsumption(len(fleet.Ships), false)
		consPerTick := consPerSec * 0.1

		if fleet.Cargo == nil {
			fleet.Cargo = make(map[domain.ResourceType]float64)
		}
		if fleet.Cargo[domain.Rum] > 0 {
			fleet.Cargo[domain.Rum] -= consPerTick
			if fleet.Cargo[domain.Rum] < 0 {
				fleet.Cargo[domain.Rum] = 0
			}
		}

		for i := range fleet.Ships {
			s := &fleet.Ships[i]

			// Calculate direction vector (Using Float)
			dx := float64(*fleet.TargetX) - s.X
			dy := float64(*fleet.TargetY) - s.Y
			dist := math.Sqrt(dx*dx + dy*dy)

			if dist > 0 {
				// Normalize and apply speed
				s.X += (dx / dist) * speed
				s.Y += (dy / dist) * speed
				db.Save(s)
			}
		}

		// Persist consumption
		db.Save(fleet)

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
		fleet.State = domain.FleetStateIdle
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
