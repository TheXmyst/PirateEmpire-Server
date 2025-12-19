package api

import (
	"fmt"
	"net/http"
	"time"

	"github.com/TheXmyst/Sea-Dogs/server/internal/auth"
	"github.com/TheXmyst/Sea-Dogs/server/internal/domain"
	"github.com/TheXmyst/Sea-Dogs/server/internal/economy"
	"github.com/TheXmyst/Sea-Dogs/server/internal/repository"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

// GetPvpTargetsResponse response structure
type GetPvpTargetsResponse struct {
	Targets []economy.PvpTarget `json:"targets"`
}

// GetPvpTargets returns a list of attackable players near the user
func GetPvpTargets(c echo.Context) error {
	// Get authenticated player
	player := auth.GetAuthenticatedPlayer(c)
	if player == nil {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "Non authentifié"})
	}

	// Load player's island to get coordinates
	db := repository.GetDB()
	var island domain.Island
	if err := db.Where("player_id = ?", player.ID).First(&island).Error; err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "Île introuvable"})
	}

	// Get targets
	targets, err := economy.GetPvpTargets(player.ID, island.X, island.Y, player.IsAdmin)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Erreur lors de la recherche de cibles"})
	}

	// Diagnostic log
	ids := make([]string, 0, len(targets))
	for _, t := range targets {
		ids = append(ids, t.IslandID.String())
	}
	fmt.Printf("[DIAGNOSTIC] requester=%s (admin=%v), count_islands_returned=%d, ids=%v\n",
		player.ID.String(), player.IsAdmin, len(targets), ids)

	return c.JSON(http.StatusOK, GetPvpTargetsResponse{
		Targets: targets,
	})
}

// AttackPvpRequest request structure
type AttackPvpRequest struct {
	TargetIslandID string `json:"target_island_id"`
	FleetID        string `json:"fleet_id"`
}

// AttackPvpResponse response structure
type AttackPvpResponse struct {
	CombatResult economy.CombatResult `json:"combat_result"`
	Loot         map[string]float64   `json:"loot"`
}

// AttackPvp handles PvP combat initiation
func AttackPvp(c echo.Context) error {
	// 1. Auth & Input Validation
	player := auth.GetAuthenticatedPlayer(c)
	if player == nil {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "Non authentifié"})
	}

	var req AttackPvpRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Requête invalide"})
	}

	attackerFleetID, err := uuid.Parse(req.FleetID)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "ID de flotte invalide"})
	}

	targetIslandID, err := uuid.Parse(req.TargetIslandID)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "ID d'île cible invalide"})
	}

	db := repository.GetDB()
	tx := db.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// 2. Load Attacker Fleet (Must own it)
	var fromIsland domain.Island
	if err := tx.Where("player_id = ?", player.ID).First(&fromIsland).Error; err != nil {
		tx.Rollback()
		return c.JSON(http.StatusNotFound, map[string]string{"error": "Votre île est introuvable"})
	}

	var attackerFleet domain.Fleet
	if err := tx.Preload("Ships").Where("id = ? AND island_id = ?", attackerFleetID, fromIsland.ID).First(&attackerFleet).Error; err != nil {
		tx.Rollback()
		return c.JSON(http.StatusNotFound, map[string]string{"error": "Flotte introuvable ou ne vous appartient pas"})
	}

	// Validate Attacker
	if isValid, _, msg := economy.ValidateFleetForCombat(&attackerFleet); !isValid {
		tx.Rollback()
		return c.JSON(http.StatusBadRequest, map[string]string{"error": msg})
	}

	// 3. Load Defender Island & Fleet (with Buildings for protection check)
	var targetIsland domain.Island
	if err := tx.Preload("Player").Preload("Buildings").First(&targetIsland, "id = ?", targetIslandID).Error; err != nil {
		tx.Rollback()
		return c.JSON(http.StatusNotFound, map[string]string{"error": "Cible introuvable"})
	}

	// Self-attack check
	if targetIsland.PlayerID == player.ID {
		tx.Rollback()
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Vous ne pouvez pas vous attaquer vous-même"})
	}

	// Protection Check (Peace Shield after defeat)
	// Skip if attacker is admin (for testing)
	if !player.IsAdmin {
		if targetIsland.ProtectedUntil != nil && time.Now().Before(*targetIsland.ProtectedUntil) {
			tx.Rollback()
			return c.JSON(http.StatusConflict, map[string]string{"error": "Ce joueur est sous protection de paix"})
		}
	}

	// Beginner Protection (TH must be level 4+ and completed)
	// Get defender TH level (needed for loot calculation later)
	defenderTH := economy.GetBuildingLevel(&targetIsland, "Hôtel de Ville")

	// Skip protection check if attacker is admin (for testing)
	if !player.IsAdmin {
		if defenderTH < economy.MinTownHallForPvP {
			// Get TH construction status for detailed error message
			thConstructing := false
			thActualLevel := 0
			for _, b := range targetIsland.Buildings {
				if b.Type == "Hôtel de Ville" {
					thActualLevel = b.Level
					thConstructing = b.Constructing
					break
				}
			}

			tx.Rollback()
			errorMsg := fmt.Sprintf("Ce joueur bénéficie de la protection débutant (TH niveau %d", thActualLevel)
			if thConstructing {
				errorMsg += " en construction"
			}
			errorMsg += fmt.Sprintf(", complété: %d, requis: %d)", defenderTH, economy.MinTownHallForPvP)
			return c.JSON(http.StatusConflict, map[string]string{"error": errorMsg})
		}
	}

	// 4. Determine Defender Logic
	// Check for Active Fleet docked at home
	var defenderFleet domain.Fleet
	var defenderCaptain *domain.Captain
	hasDefenderFleet := false

	if targetIsland.ActiveFleetID != nil {
		var f domain.Fleet
		if err := tx.Preload("Ships").First(&f, "id = ?", *targetIsland.ActiveFleetID).Error; err == nil {
			// Check if fleet is available (not locked, not away)
			if f.LockedUntil == nil || time.Now().After(*f.LockedUntil) {
				// Fleet defends!
				defenderFleet = f
				hasDefenderFleet = true

				// Get Captain
				flagship, _, _ := economy.SelectFlagshipShip(&f)
				if flagship != nil && flagship.CaptainID != nil {
					var c domain.Captain
					if err := tx.First(&c, "id = ?", *flagship.CaptainID).Error; err == nil {
						defenderCaptain = &c
					}
				}
			}
		}
	}

	// If no fleet, generate Militia Fleet (Abstract representation)
	if !hasDefenderFleet {
		// Verify militia count
		// For now simple implementation: Tier 1 NPC Fleet equivalent if Militia > 0
		// Better: Generate Militia Fleet based on targetIsland.MilitiaWarriors...
		// V1 Simplification: Generate Weak Militia Fleet based on TH level
		defenderFleet = economy.GenerateNpcFleet(1, "militia_"+targetIslandID.String())
		defenderFleet.Name = "Milice locale"
	}

	// 5. Execute Combat
	// Get Attacker Captain
	var attackerCaptain *domain.Captain
	flagshipA, _, _ := economy.SelectFlagshipShip(&attackerFleet)
	if flagshipA != nil && flagshipA.CaptainID != nil {
		var c domain.Captain
		if err := tx.First(&c, "id = ?", *flagshipA.CaptainID).Error; err == nil {
			attackerCaptain = &c
		}
	}

	// Simulate Engagement
	engResult := economy.ComputeEngagementMorale(attackerFleet, defenderFleet, attackerCaptain, defenderCaptain)

	// Fight!
	combatRes, err := economy.ExecuteNavalCombat(&attackerFleet, &defenderFleet, attackerCaptain, defenderCaptain, engResult, time.Now().UnixNano())
	if err != nil {
		tx.Rollback()
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Erreur combat"})
	}

	// 6. Apply Results
	// Destroy ships hard deletion (Attacker only - Defender real ships only if fleet defended)
	for _, id := range combatRes.ShipsDestroyedA {
		economy.DestroyShipHard(tx, id)
	}
	if hasDefenderFleet {
		for _, id := range combatRes.ShipsDestroyedB {
			economy.DestroyShipHard(tx, id)
		}
	}

	// 7. Loot Logic (Winner = Fleet A)
	loot := make(map[string]float64)
	if combatRes.Winner == "fleet_a" {
		// Calculate Safe Amount
		safeAmount := float64(defenderTH * 1000)

		// Steal 50% of vulnerable
		for res, amount := range targetIsland.Resources {
			available := amount - safeAmount
			if available > 0 {
				stolen := available * 0.50
				// TODO: Check fleet capacity (ignored for V1 simplicity)
				loot[string(res)] = stolen

				// Transfer
				targetIsland.Resources[res] -= stolen
				fromIsland.Resources[res] += stolen
			}
		}

		// Apply Peace Shield to Defender (4 hours)
		shieldEnd := time.Now().Add(4 * time.Hour)
		targetIsland.ProtectedUntil = &shieldEnd

		// Fleet locking removed - fleets are immediately available after combat
	}

	// Save everything
	if err := tx.Save(&targetIsland).Error; err != nil {
		tx.Rollback()
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Erreur sauvegarde cible"})
	}
	if err := tx.Save(&fromIsland).Error; err != nil {
		tx.Rollback()
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Erreur sauvegarde attaquant"})
	}

	tx.Commit()

	return c.JSON(http.StatusOK, AttackPvpResponse{
		CombatResult: combatRes,
		Loot:         loot,
	})
}
