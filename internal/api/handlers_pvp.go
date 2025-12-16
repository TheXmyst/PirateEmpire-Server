package api

import (
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
	targets, err := economy.GetPvpTargets(player.ID, island.X, island.Y)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Erreur lors de la recherche de cibles"})
	}

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

	// 3. Load Defender Island & Fleet
	var targetIsland domain.Island
	if err := tx.Preload("Player").First(&targetIsland, "id = ?", targetIslandID).Error; err != nil {
		tx.Rollback()
		return c.JSON(http.StatusNotFound, map[string]string{"error": "Cible introuvable"})
	}

	// Self-attack check
	if targetIsland.PlayerID == player.ID {
		tx.Rollback()
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Vous ne pouvez pas vous attaquer vous-même"})
	}

	// Protection Check
	if targetIsland.ProtectedUntil != nil && time.Now().Before(*targetIsland.ProtectedUntil) {
		tx.Rollback()
		return c.JSON(http.StatusConflict, map[string]string{"error": "Ce joueur est sous protection de paix"})
	}

	// Beginner Protection
	defenderTH := economy.GetBuildingLevel(&targetIsland, "Hôtel de Ville")
	if defenderTH < 3 {
		tx.Rollback()
		return c.JSON(http.StatusConflict, map[string]string{"error": "Ce joueur bénéficie de la protection débutant"})
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
		// Better: Generate Militia Fleet based on targetIsland.CrewWarriors...
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

		// Lock Attacker Fleet (Cooldown / Return trip)
		lockEnd := time.Now().Add(5 * time.Minute) // 5 min travel back
		attackerFleet.LockedUntil = &lockEnd
		tx.Save(&attackerFleet)
	} else {
		// Defender Won -> Attacker flees (Locked for return trip)
		lockEnd := time.Now().Add(10 * time.Minute) // 10 min "limping" back
		attackerFleet.LockedUntil = &lockEnd
		tx.Save(&attackerFleet)
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
