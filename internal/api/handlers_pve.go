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

// GetPveTargetsRequest is the request for GET /pve/targets
// No body needed, player ID comes from auth context

// GetPveTargetsResponse is the response for GET /pve/targets
type GetPveTargetsResponse struct {
	Targets []economy.PveTarget `json:"targets"`
}

// GetPveTargets returns 3 PVE targets for the authenticated player's island
func GetPveTargets(c echo.Context) error {
	// Get authenticated player
	player := auth.GetAuthenticatedPlayer(c)
	if player == nil {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "Non authentifié"})
	}
	playerID := player.ID

	db := repository.GetDB()

	// Load player's island
	var island domain.Island
	if err := db.Where("player_id = ?", playerID).First(&island).Error; err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "Île introuvable"})
	}

	// Get PVE targets (from cache or generate new)
	targets := economy.GetPveTargets(playerID, island.X, island.Y)

	return c.JSON(http.StatusOK, GetPveTargetsResponse{
		Targets: targets,
	})
}

// EngagePveRequest is the request for POST /pve/engage
type EngagePveRequest struct {
	FleetID  string `json:"fleet_id"`       // UUID as string
	TargetID string `json:"target_id"`      // e.g., "npc-<playerID>-<slotIndex>"
	Seed     *int64 `json:"seed,omitempty"` // Optional: deterministic RNG seed
}

// EngagePveResponse is the response for POST /pve/engage
type EngagePveResponse struct {
	CombatResult economy.CombatResult `json:"combat_result"`
	Rewards      *PveRewards          `json:"rewards,omitempty"` // Optional rewards (v1: none)
}

// PveRewards represents rewards from PVE combat (v1: minimal)
type PveRewards struct {
	Gold  float64 `json:"gold,omitempty"`
	Wood  float64 `json:"wood,omitempty"`
	Stone float64 `json:"stone,omitempty"`
	Rum   float64 `json:"rum,omitempty"`
}

// EngagePve handles PVE engagement: locks fleet, generates NPC fleet, executes combat, applies results
func EngagePve(c echo.Context) error {
	// Get authenticated player
	player := auth.GetAuthenticatedPlayer(c)
	if player == nil {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "Non authentifié"})
	}
	playerID := player.ID

	// Parse request
	req := new(EngagePveRequest)
	if err := c.Bind(req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": fmt.Sprintf("Requête invalide: %v", err)})
	}

	// Validate fleet_id
	if req.FleetID == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "fleet_id manquant"})
	}
	fleetID, err := uuid.Parse(req.FleetID)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": fmt.Sprintf("fleet_id invalide (UUID attendu): '%s'", req.FleetID)})
	}

	// Validate target_id
	if req.TargetID == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "target_id manquant"})
	}

	// Get tier from cache
	tier := 1 // Default tier
	target := economy.GetPveTargetByID(playerID, req.TargetID)
	if target != nil {
		tier = target.Tier
	} else {
		// Fallback: try to parse from target_id (format: "npc-<playerID>-<slotIndex>")
		// Slot index 0 = tier 1, 1 = tier 2, 2 = tier 3
		if len(req.TargetID) > 0 {
			lastChar := req.TargetID[len(req.TargetID)-1:]
			switch lastChar {
			case "0":
				tier = 1
			case "1":
				tier = 2
			case "2":
				tier = 3
			default:
				tier = 1
			}
		}
	}

	db := repository.GetDB()

	// Start transaction
	tx := db.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
			fmt.Printf("[PVE] EngagePve: Panic recovered: %v\n", r)
		}
	}()

	// Load player's island
	var island domain.Island
	if err := tx.Where("player_id = ?", playerID).First(&island).Error; err != nil {
		tx.Rollback()
		return c.JSON(http.StatusNotFound, map[string]string{"error": "Île introuvable"})
	}

	// Load fleet with ships and verify ownership
	var fleet domain.Fleet
	if err := tx.Preload("Ships").Where("id = ? AND island_id = ?", fleetID, island.ID).First(&fleet).Error; err != nil {
		tx.Rollback()
		return c.JSON(http.StatusNotFound, map[string]string{"error": "Flotte introuvable ou ne vous appartient pas"})
	}

	// Validate fleet for combat (includes crew validation)
	isValid, reasonCode, reasonMsg := economy.ValidateFleetForCombat(&fleet)
	if !isValid {
		tx.Rollback()
		statusCode := http.StatusBadRequest
		if reasonCode == "FLEET_LOCKED" {
			statusCode = http.StatusConflict
		}
		// Log validation failure
		fmt.Printf("[PVE] validate crew failed fleet=%s code=%s msg=%s\n", fleet.ID.String()[:8], reasonCode, reasonMsg)
		return c.JSON(statusCode, map[string]interface{}{
			"error":          reasonMsg,
			"reason_code":    reasonCode,
			"reason_message": reasonMsg,
		})
	}

	// Log successful validation
	activeShipsCount := 0
	for _, ship := range fleet.Ships {
		if ship.State != "Destroyed" && ship.Health > 0 {
			activeShipsCount++
		}
	}
	fmt.Printf("[PVE] validate crew ok fleet=%s ships=%d\n", fleet.ID.String()[:8], activeShipsCount)

	// Lock fleet for 60 seconds
	lockDuration := 60 * time.Second
	lockUntil := time.Now().Add(lockDuration)
	fleet.LockedUntil = &lockUntil
	if err := tx.Save(&fleet).Error; err != nil {
		tx.Rollback()
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Erreur lors du verrouillage de la flotte"})
	}

	// Generate NPC fleet based on tier
	npcFleet := economy.GenerateNpcFleet(tier, req.TargetID)

	// Load captain for player fleet (if flagship has one)
	var captA *domain.Captain
	flagshipA, _, _ := economy.SelectFlagshipShip(&fleet)
	if flagshipA != nil && flagshipA.CaptainID != nil {
		var captain domain.Captain
		if err := tx.First(&captain, "id = ?", *flagshipA.CaptainID).Error; err == nil {
			captA = &captain
			// Validate captain for combat
			isValid, reasonCode, reasonMsg := economy.ValidateFleetCaptainForCombat(captA)
			if !isValid {
				tx.Rollback()
				return c.JSON(http.StatusBadRequest, map[string]string{"error": reasonMsg, "reason_code": reasonCode})
			}
		}
	}

	// NPC fleet has no captain (v1)
	var captB *domain.Captain = nil

	// Compute engagement morale
	engagementResult := economy.ComputeEngagementMorale(fleet, npcFleet, captA, captB)

	// Generate seed if not provided
	seed := time.Now().UnixNano()
	if req.Seed != nil {
		seed = *req.Seed
	}

	// Execute combat
	combatResult, err := economy.ExecuteNavalCombat(
		&fleet, &npcFleet,
		captA, captB,
		engagementResult,
		seed,
	)
	if err != nil {
		tx.Rollback()
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": fmt.Sprintf("Erreur lors du combat: %v", err)})
	}

	// Apply combat results: hard delete destroyed ships, injure captain
	// Ships destroyed from player fleet - hard delete them
	for _, destroyedShipID := range combatResult.ShipsDestroyedA {
		if err := economy.DestroyShipHard(tx, destroyedShipID); err != nil {
			tx.Rollback()
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": fmt.Sprintf("Échec suppression navire détruit: %v", err)})
		}
	}

	// Captain injury (if flagship destroyed)
	if combatResult.CaptainInjuredA != nil && captA != nil {
		injuryDuration := economy.GetCaptainInjuryDuration(captA.Rarity)
		injuredUntil := time.Now().Add(injuryDuration)
		captA.InjuredUntil = &injuredUntil
		if err := tx.Save(captA).Error; err != nil {
			fmt.Printf("[PVE] Failed to apply captain injury: %v\n", err)
		}
	}

	// Consume target from cache
	economy.ConsumePveTarget(playerID, req.TargetID)

	// Commit transaction
	if err := tx.Commit().Error; err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Erreur lors de la sauvegarde"})
	}

	fmt.Printf("[PVE] EngagePve: player=%s fleet=%s target=%s tier=%d winner=%s ships_destroyed=%d\n",
		playerID, fleetID, req.TargetID, tier, combatResult.Winner, len(combatResult.ShipsDestroyedA))

	// Build response
	response := EngagePveResponse{
		CombatResult: combatResult,
		// Rewards: nil for v1 (no rewards yet)
	}

	return c.JSON(http.StatusOK, response)
}
