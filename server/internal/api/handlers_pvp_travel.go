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

// SendPvpAttackRequest payload for sending fleet to attack
type SendPvpAttackRequest struct {
	FleetID        string `json:"fleet_id"`
	TargetIslandID string `json:"target_island_id"`
}

// SendPvpAttackResponse response for send attack
type SendPvpAttackResponse struct {
	TravelTimeMinutes float64 `json:"travel_time_minutes"`
	Distance          float64 `json:"distance"`
	Message           string  `json:"message"`
}

// SendPvpAttack initiates fleet travel to enemy island
func SendPvpAttack(c echo.Context) error {
	// Get authenticated player
	player := auth.GetAuthenticatedPlayer(c)
	if player == nil {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "Non authentifié"})
	}

	// Parse request
	var req SendPvpAttackRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid request"})
	}

	// Validate UUIDs
	fleetID, err := uuid.Parse(req.FleetID)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid fleet ID"})
	}

	targetIslandID, err := uuid.Parse(req.TargetIslandID)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid target island ID"})
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
	if fleet.State != domain.FleetStateIdle {
		return c.JSON(http.StatusConflict, map[string]string{"error": "La flotte n'est pas disponible"})
	}

	// Check fleet has ships
	if len(fleet.Ships) == 0 {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "La flotte est vide"})
	}

	// Load target island
	var targetIsland domain.Island
	if err := db.Where("id = ?", targetIslandID).First(&targetIsland).Error; err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "Île cible introuvable"})
	}

	// Calculate distance
	distance := economy.CalculateDistance(island.X, island.Y, targetIsland.X, targetIsland.Y)

	// Calculate travel time (50 units per minute)
	travelTimeMinutes := distance / 50.0

	// Update fleet state
	oldState := fleet.State
	fleet.State = domain.FleetStateTravelingToAttack
	fmt.Printf("[FLEET_STATE] fleet=%s from=%s to=%s reason=SendPvpAttack\n", fleet.ID, oldState, fleet.State)

	fleet.TargetIslandID = &targetIslandID
	fleet.TargetX = &targetIsland.X
	fleet.TargetY = &targetIsland.Y

	if err := db.Save(&fleet).Error; err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Erreur sauvegarde flotte"})
	}

	// Checkpoint: Log success
	fmt.Printf("[PVP_TRAVEL] send_attack requester=%s fleet=%s target_island=%s ok\n", player.ID, fleet.ID, targetIsland.ID)

	return c.JSON(http.StatusOK, SendPvpAttackResponse{
		TravelTimeMinutes: travelTimeMinutes,
		Distance:          distance,
		Message:           fmt.Sprintf("Flotte en route vers %s (ETA: %.1f min)", targetIsland.Name, travelTimeMinutes),
	})
}
