package api

import (
	"fmt"
	"net/http"
	"time"

	"github.com/TheXmyst/Sea-Dogs/server/internal/auth"
	"github.com/TheXmyst/Sea-Dogs/server/internal/domain"
	"github.com/TheXmyst/Sea-Dogs/server/internal/repository"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"gorm.io/gorm"
)

type InterceptRequest struct {
	AttackerFleetID string `json:"attacker_fleet_id"`
	TargetFleetID   string `json:"target_fleet_id"`
}

// StartIntercept initiates a real-time pursuit
func StartIntercept(c echo.Context) error {
	player := auth.GetAuthenticatedPlayer(c)
	if player == nil {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "Non authentifié"})
	}

	var req InterceptRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Requête invalide"})
	}

	attackerID, err := uuid.Parse(req.AttackerFleetID)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "ID attaquant invalide"})
	}

	targetID, err := uuid.Parse(req.TargetFleetID)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "ID cible invalide"})
	}

	db := repository.GetDB()
	var attackerFleet domain.Fleet
	var targetFleet domain.Fleet

	// 1. Core Validations
	if err := db.Preload("Island").First(&attackerFleet, "id = ?", attackerID).Error; err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "Flotte attaquante introuvable"})
	}
	if attackerFleet.Island.PlayerID != player.ID {
		return c.JSON(http.StatusForbidden, map[string]string{"error": "Cette flotte ne vous appartient pas"})
	}

	if err := db.Preload("Island").First(&targetFleet, "id = ?", targetID).Error; err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "Flotte cible introuvable"})
	}

	if targetFleet.Island.PlayerID == player.ID {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Vous ne pouvez pas intercepter votre propre flotte"})
	}

	// 2. State & Location Validations
	if !attackerFleet.IsAtSea() {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "La flotte doit être en mer pour intercepter"})
	}
	if !targetFleet.IsAtSea() {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "La cible doit être en mer pour être interceptée"})
	}

	// CanTransitionTo logic placeholder (we'll implement the method in models.go if not exists or check manually)
	if attackerFleet.State == domain.FleetStateChasingPvP || attackerFleet.State == domain.FleetStateTravelingToAttack {
		return c.JSON(http.StatusConflict, map[string]string{"error": "Flotte déjà engagée dans une mission"})
	}

	// 3. Cooldown Check (Anti-Harassment)
	var cooldown domain.PvPInterceptCooldown
	now := time.Now()
	if err := db.Where("attacker_player_id = ? AND target_player_id = ? AND blocked_until > ?", player.ID, targetFleet.Island.PlayerID, now).First(&cooldown).Error; err == nil {
		diff := cooldown.BlockedUntil.Sub(now)
		return c.JSON(http.StatusConflict, map[string]string{"error": fmt.Sprintf("Interception indisponible (cooldown: %d min)", int(diff.Minutes())+1)})
	}

	// 4. Execute Start
	err = db.Transaction(func(tx *gorm.DB) error {
		// Update Attacker
		attackerFleet.State = domain.FleetStateChasingPvP
		attackerFleet.ChasingFleetID = &targetID
		attackerFleet.InterceptStartedAt = &now
		attackerFleet.InterceptTargetPlayerID = &targetFleet.Island.PlayerID

		if err := tx.Save(&attackerFleet).Error; err != nil {
			return err
		}

		// Update Target (Soft link for UI awareness)
		targetFleet.ChasedByFleetID = &attackerID
		if err := tx.Save(&targetFleet).Error; err != nil {
			return err
		}

		return nil
	})

	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Erreur lors du lancement de l'interception"})
	}

	fmt.Printf("[PVP_INTERCEPT] start attacker_fleet=%s (player=%s) target_fleet=%s (player=%s)\n",
		attackerID, player.Username, targetID, targetFleet.Island.Player.Username)

	return c.JSON(http.StatusOK, map[string]string{"message": "Interception lancée ! Poursuite en cours..."})
}

// AbortIntercept cancels a pursuit from the attacker side
func AbortIntercept(c echo.Context) error {
	player := auth.GetAuthenticatedPlayer(c)
	if player == nil {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "Non authentifié"})
	}

	var req struct {
		AttackerFleetID string `json:"attacker_fleet_id"`
	}
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Requête invalide"})
	}

	fleetID, _ := uuid.Parse(req.AttackerFleetID)
	db := repository.GetDB()

	var fleet domain.Fleet
	if err := db.Preload("Island").First(&fleet, "id = ?", fleetID).Error; err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "Flotte introuvable"})
	}

	if fleet.Island.PlayerID != player.ID {
		return c.JSON(http.StatusForbidden, map[string]string{"error": "Accès refusé"})
	}

	if fleet.State != domain.FleetStateChasingPvP {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "La flotte n'est pas en train d'intercepter"})
	}

	targetFleetID := fleet.ChasingFleetID

	err := db.Transaction(func(tx *gorm.DB) error {
		// Clear Attacker
		fleet.State = domain.FleetStateSeaStationed // Default to sea-stationed if aborted at sea
		fleet.ChasingFleetID = nil
		fleet.InterceptStartedAt = nil
		fleet.InterceptTargetPlayerID = nil
		if err := tx.Save(&fleet).Error; err != nil {
			return err
		}

		// Clear Target if possible
		if targetFleetID != nil {
			tx.Model(&domain.Fleet{}).Where("id = ?", *targetFleetID).Update("chased_by_fleet_id", nil)
		}

		return nil
	})

	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Erreur lors de l'abandon"})
	}

	fmt.Printf("[PVP_INTERCEPT] abort attacker_fleet=%s\n", fleetID)
	return c.JSON(http.StatusOK, map[string]string{"message": "Interception abandonnée"})
}
