package api

import (
	"fmt"
	"net/http"

	"github.com/TheXmyst/Sea-Dogs/server/internal/auth"
	"github.com/TheXmyst/Sea-Dogs/server/internal/domain"
	"github.com/TheXmyst/Sea-Dogs/server/internal/economy"
	"github.com/TheXmyst/Sea-Dogs/server/internal/repository"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

// SendPveAttackRequest payload for sending fleet to attack NPC
type SendPveAttackRequest struct {
	FleetID     string `json:"fleet_id"`
	TargetPveID string `json:"target_pve_id"` // UUID of the NPC target
}

// SendPveAttackResponse response for send pve attack
type SendPveAttackResponse struct {
	Message string `json:"message"`
}

// SendPveAttack initiates fleet chase on moving NPC
func SendPveAttack(c echo.Context) error {
	// Get authenticated player
	player := auth.GetAuthenticatedPlayer(c)
	if player == nil {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "Non authentifié"})
	}

	// Parse request
	var req SendPveAttackRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid request"})
	}

	// Validate UUIDs
	fleetID, err := uuid.Parse(req.FleetID)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid fleet ID"})
	}

	targetPveID, err := uuid.Parse(req.TargetPveID)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid target PVE ID"})
	}

	db := repository.GetDB()

	// Load fleet with ships
	var fleet domain.Fleet
	if err := db.Preload("Ships").Where("id = ?", fleetID).First(&fleet).Error; err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "Flotte introuvable"})
	}

	// Verify ownership
	var island domain.Island
	if err := db.Where("id = ?", fleet.IslandID).First(&island).Error; err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "Île introuvable"})
	}

	if island.PlayerID != player.ID {
		return c.JSON(http.StatusForbidden, map[string]string{"error": "Cette flotte ne vous appartient pas"})
	}

	// Check fleet is idle
	if fleet.State != "Idle" {
		return c.JSON(http.StatusConflict, map[string]string{"error": "La flotte n'est pas disponible"})
	}

	// Check fleet has ships
	if len(fleet.Ships) == 0 {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "La flotte est vide"})
	}

	// Load PvE target from cache
	// IMPORTANT: Use the new UUID lookup
	pveTarget := economy.GetPveTargetByUUID(player.ID, targetPveID)
	if pveTarget == nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "Cible PvE introuvable ou expirée"})
	}

	// Initial Position Update
	tx := int(pveTarget.RealX)
	ty := int(pveTarget.RealY)

	// Update fleet state
	fleet.State = "Chasing_PvE"
	fleet.TargetPveID = &targetPveID
	fleet.TargetX = &tx
	fleet.TargetY = &ty

	// Clear other target types
	fleet.TargetIslandID = nil

	if err := db.Save(&fleet).Error; err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Erreur sauvegarde flotte"})
	}

	// Log (Minimal)
	fmt.Printf("[PVE_TRACK] start fleet=%s target=%s (uuid=%s)\n", fleet.ID, pveTarget.Name, targetPveID)

	return c.JSON(http.StatusOK, SendPveAttackResponse{
		Message: fmt.Sprintf("À l'abordage ! Poursuite de %s engagée.", pveTarget.Name),
	})
}
