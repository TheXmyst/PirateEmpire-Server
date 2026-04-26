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

// StationFleetRequest payload
type StationFleetRequest struct {
	FleetID string `json:"fleet_id"`
	NodeID  string `json:"node_id"`
}

// StationFleet initiates the journey to a resource node
func StationFleet(c echo.Context) error {
	player := auth.GetAuthenticatedPlayer(c)
	if player == nil {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "Unauthorized"})
	}

	var req StationFleetRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid payload"})
	}

	fleetID, err := uuid.Parse(req.FleetID)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid Fleet ID"})
	}

	db := repository.GetDB()
	var island domain.Island
	// Preload fleets and ships
	if err := db.Preload("Fleets.Ships").Where("player_id = ?", player.ID).First(&island).Error; err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "Island not found"})
	}

	// Validate Fleet
	var fleet *domain.Fleet
	for i := range island.Fleets {
		if island.Fleets[i].ID == fleetID {
			fleet = &island.Fleets[i]
			break
		}
	}

	if fleet == nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "Fleet not found"})
	}

	// Must be Idle OR Returning
	if fleet.StationedNodeID != nil || fleet.State == domain.FleetStateStationed {
		return c.JSON(http.StatusConflict, map[string]string{"error": "Fleet is already stationed"})
	}

	// Find the node
	node := economy.GetResourceNodeByID(player.ID, req.NodeID, island.X, island.Y)
	if node == nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Resource Node invalid or expired"})
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
	fmt.Printf("[FLEET_STATE] fleet=%s from=%s to=%s reason=StationFleet\n", fleet.ID, oldState, fleet.State)

	fleet.StationedNodeID = &node.ID
	fleet.StoredResource = string(node.Type)
	fleet.StoredAmount = 0

	// Set Target Coordinates from Node
	targetX := int(node.X)
	targetY := int(node.Y)
	fleet.TargetX = &targetX
	fleet.TargetY = &targetY

	// Initialize ship positions if they are starting from "Idle"
	for i := range fleet.Ships {
		s := &fleet.Ships[i]
		if s.State != "UnderConstruction" {
			// FORCE SNAP if Idle (to avoid starting from 0,0 if uninitialized)
			if fleet.State == domain.FleetStateIdle || (s.X == 0 && s.Y == 0) {
				s.X = float64(island.X)
				s.Y = float64(island.Y)
			}
			if err := tx.Save(s).Error; err != nil {
				tx.Rollback()
				return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to initialize ship coordinates"})
			}
		}
	}

	if err := tx.Save(fleet).Error; err != nil {
		tx.Rollback()
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to save fleet"})
	}

	tx.Commit()

	return c.JSON(http.StatusOK, map[string]string{
		"status":   "Fleet sent to station",
		"state":    "Moving",
		"target_x": fmt.Sprintf("%d", targetX),
		"target_y": fmt.Sprintf("%d", targetY),
	})
}

// RecallFleet brings the fleet home
func RecallFleet(c echo.Context) error {
	player := auth.GetAuthenticatedPlayer(c)
	if player == nil {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "Unauthorized"})
	}

	// Use Body or Query? Usually POST /recall with fleet_id in payload or URL
	// Trying Bind for payload
	var req struct {
		FleetID string `json:"fleet_id"`
	}
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid payload"})
	}

	fleetID, err := uuid.Parse(req.FleetID)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid Fleet ID"})
	}

	db := repository.GetDB()
	var fleet domain.Fleet

	// Load fleet and verify ownership via island join logic
	if err := db.Preload("Ships").First(&fleet, "id = ?", fleetID).Error; err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "Fleet not found"})
	}

	var island domain.Island
	if err := db.First(&island, "id = ?", fleet.IslandID).Error; err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Island link broken"})
	}
	if island.PlayerID != player.ID {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "Not your fleet"})
	}

	// If already Returning or Idle, error
	if fleet.State == domain.FleetStateReturning || fleet.State == domain.FleetStateIdle {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Fleet is not stationed or moving"})
	}

	// Trigger Return
	tx := db.Begin()
	oldState := fleet.State
	fleet.State = domain.FleetStateReturning
	fleet.FreeNav = false
	fmt.Printf("[FLEET_STATE] fleet=%s from=%s to=%s reason=RecallFleet free_nav=false\n", fleet.ID, oldState, fleet.State)

	// Set Target to Home
	homeX := island.X
	homeY := island.Y
	fleet.TargetX = &homeX
	fleet.TargetY = &homeY

	// NOTE: We do NOT clear StoredAmount here. It's carried home.
	// We clear StationedNodeID/At to stop gathering.
	fleet.StationedNodeID = nil
	fleet.StationedAt = nil

	if err := tx.Save(&fleet).Error; err != nil {
		tx.Rollback()
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to recall fleet"})
	}

	// Also ensure ship positions are saved if updated briefly
	for i := range fleet.Ships {
		tx.Save(&fleet.Ships[i])
	}
	tx.Commit()

	return c.JSON(http.StatusOK, map[string]string{
		"status":            "Fleet returning",
		"resources_carried": fmt.Sprintf("%.0f", fleet.StoredAmount),
	})
}

// GetResourceNodesResponse format
type GetResourceNodesResponse struct {
	Nodes []domain.ResourceNode `json:"nodes"`
}

// GetResourceNodes fetches visible nodes for the player
func GetResourceNodes(c echo.Context) error {
	player := auth.GetAuthenticatedPlayer(c)
	if player == nil {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "Unauthorized"})
	}

	db := repository.GetDB()
	var island domain.Island
	if err := db.Where("player_id = ?", player.ID).First(&island).Error; err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "Island not found"})
	}

	// Minimal fetch for collision (just x,y) to pass to generation
	// (though GetResourceNodes logic might not even use it if cached, but good to have)
	var islands []domain.Island
	db.Select("x", "y").Find(&islands)

	nodes := economy.GetResourceNodes(player.ID, island.X, island.Y, islands)

	return c.JSON(http.StatusOK, GetResourceNodesResponse{Nodes: nodes})
}

// GetWeather returns current wind conditions from the engine
func GetWeather(c echo.Context) error {
	gw := economy.GlobalWeather
	if gw == nil {
		return c.JSON(http.StatusOK, map[string]interface{}{
			"direction":   0.0,
			"next_change": "2099-01-01T00:00:00Z",
		})
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"direction":   gw.WindDirection,
		"next_change": gw.NextChange.Format(time.RFC3339),
	})
}
