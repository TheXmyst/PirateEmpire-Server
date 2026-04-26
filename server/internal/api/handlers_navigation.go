package api

import (
	"fmt"
	"net/http"

	"github.com/TheXmyst/Sea-Dogs/server/internal/auth"
	"github.com/TheXmyst/Sea-Dogs/server/internal/domain"
	"github.com/TheXmyst/Sea-Dogs/server/internal/repository"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

type NavigateFleetRequest struct {
	FleetID string `json:"fleet_id"`
	TargetX int    `json:"target_x"`
	TargetY int    `json:"target_y"`
}

// NavigateFleet handles free navigation requests
func NavigateFleet(c echo.Context) error {
	player := auth.GetAuthenticatedPlayer(c)
	if player == nil {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "Unauthorized"})
	}

	var req NavigateFleetRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid payload"})
	}

	fleetID, err := uuid.Parse(req.FleetID)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid Fleet ID"})
	}

	db := repository.GetDB()
	var fleet domain.Fleet
	if err := db.Preload("Ships").First(&fleet, "id = ?", fleetID).Error; err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "Fleet not found"})
	}

	// Validate Ownership
	var island domain.Island
	if err := db.First(&island, "id = ?", fleet.IslandID).Error; err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Island link broken"})
	}
	if island.PlayerID != player.ID {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "Not your fleet"})
	}

	// Validate current state (Locked states block navigation)
	if fleet.BlocksOrders() {
		return c.JSON(http.StatusConflict, map[string]string{"error": "Fleet is busy with a combat mission"})
	}

	// Update Fleet State and Target
	tx := db.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	oldState := fleet.State
	fleet.State = domain.FleetStateMoving
	fleet.FreeNav = true

	// Set Target
	txCoord := req.TargetX
	tyCoord := req.TargetY
	fleet.TargetX = &txCoord
	fleet.TargetY = &tyCoord

	// If leaving dock/node
	fleet.StationedNodeID = nil
	fleet.StationedAt = nil

	// Log transition
	fmt.Printf("[NAV_FREE] fleet=%s from=%s to=%s state_before=%s state_after=%s target=(%d,%d)\n",
		fleet.ID, oldState, fleet.State, oldState, fleet.State, txCoord, tyCoord)

	// Initialize ship positions if they are starting from "Idle" or uninitialized
	for i := range fleet.Ships {
		s := &fleet.Ships[i]
		if (s.X == 0 && s.Y == 0) || (oldState == domain.FleetStateIdle && s.X == float64(island.X) && s.Y == float64(island.Y)) {
			// Ensure they start from the island if they were just sitting there
			s.X = float64(island.X)
			s.Y = float64(island.Y)
		}
		if err := tx.Save(s).Error; err != nil {
			tx.Rollback()
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to sync ship positions"})
		}
	}

	if err := tx.Save(&fleet).Error; err != nil {
		tx.Rollback()
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to save fleet state"})
	}

	tx.Commit()

	return c.JSON(http.StatusOK, map[string]interface{}{
		"status":   "Course set",
		"state":    fleet.State,
		"target_x": txCoord,
		"target_y": tyCoord,
	})
}
