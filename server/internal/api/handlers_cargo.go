package api

import (
	"fmt"
	"net/http"

	"github.com/TheXmyst/Sea-Dogs/server/internal/domain"
	"github.com/TheXmyst/Sea-Dogs/server/internal/logger"
	"github.com/TheXmyst/Sea-Dogs/server/internal/repository"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"gorm.io/gorm"
)

// CargoTransferRequest represents the payload for transfer actions
type CargoTransferRequest struct {
	FleetID  uuid.UUID           `json:"fleet_id"`
	Resource domain.ResourceType `json:"resource"`
	Amount   float64             `json:"amount"`
}

// CargoTransferResponse returns the updated state
type CargoTransferResponse struct {
	FleetCargo      map[domain.ResourceType]float64 `json:"fleet_cargo"`
	IslandResources map[domain.ResourceType]float64 `json:"island_resources"`
	CargoCapacity   float64                         `json:"cargo_capacity"`
	CargoUsed       float64                         `json:"cargo_used"`
	CargoFree       float64                         `json:"cargo_free"`
	Message         string                          `json:"message"`
}

// TransferToFleet handles moving resources from Island to Fleet Cargo
func TransferToFleet(c echo.Context) error {
	var req CargoTransferRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid request body"})
	}

	if req.Amount <= 0 {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Amount must be positive"})
	}

	islandIDStr := c.QueryParam("island_id")
	if islandIDStr == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Missing island_id query param"})
	}
	islandID, err := uuid.Parse(islandIDStr)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid island_id"})
	}

	db := repository.GetDB()
	var fleet domain.Fleet
	var island domain.Island

	// Transaction ensuring atomicity
	err = db.Transaction(func(tx *gorm.DB) error {
		// Lock island
		if err := tx.Preload("Player").First(&island, "id = ?", islandID).Error; err != nil {
			return err
		}

		if island.Resources == nil {
			island.Resources = make(map[domain.ResourceType]float64)
		}

		// Check Island Balance
		available := island.Resources[req.Resource]
		if available < req.Amount {
			return fmt.Errorf("insufficient island resources: have %.0f, need %.0f", available, req.Amount)
		}

		// Lock fleet
		if err := tx.Preload("Ships").First(&fleet, "id = ? AND island_id = ?", req.FleetID, islandID).Error; err != nil {
			return err
		}

		// Verify Fleet State
		if fleet.State != "Idle" && fleet.State != "Stationed" {
			return fmt.Errorf("fleet must be Idle or Stationed to transfer cargo (current: %s)", fleet.State)
		}

		// SSOT: Compute Capacity immediately
		fleet.ComputePayload()

		// Check Capacity
		if fleet.CargoLoaded()+req.Amount > fleet.CargoCapacity {
			// Debug Log (Requested for diagnosis)
			logger.Warn("[CARGO] transfer refused",
				"fleet", fleet.Name,
				"ships", len(fleet.Ships),
				"cap", fleet.CargoCapacity,
				"used", fleet.CargoLoaded(),
				"free", fleet.CargoCapacity-fleet.CargoLoaded(),
				"need", req.Amount,
				"cargo", fleet.Cargo)
			return fmt.Errorf("insufficient fleet capacity: available %.0f, need %.0f", fleet.CargoCapacity-fleet.CargoLoaded(), req.Amount)
		}

		// EXECUTE TRANSFER
		island.Resources[req.Resource] -= req.Amount
		fleet.Cargo[req.Resource] += req.Amount

		// Update transient fields for response
		fleet.ComputePayload()

		// Save hook handles JSON serialization
		if err := tx.Save(&island).Error; err != nil {
			return err
		}
		if err := tx.Save(&fleet).Error; err != nil {
			return err
		}

		logger.Info("[CARGO] Transfer Island/Fleet",
			"player", island.Player.Username,
			"fleet", fleet.Name,
			"res", req.Resource,
			"amount", req.Amount)

		return nil
	})

	if err != nil {
		return c.JSON(http.StatusConflict, map[string]string{"error": err.Error()})
	}

	// Success Response
	resp := CargoTransferResponse{
		FleetCargo:      fleet.Cargo,
		IslandResources: island.Resources,
		CargoCapacity:   fleet.CargoCapacity,
		CargoUsed:       fleet.CargoUsed,
		CargoFree:       fleet.CargoFree,
		Message:         fmt.Sprintf("Transferred %.0f %s to fleet", req.Amount, req.Resource),
	}
	return c.JSON(http.StatusOK, resp)
}

// TransferToIsland handles moving resources from Fleet Cargo to Island
func TransferToIsland(c echo.Context) error {
	var req CargoTransferRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid request body"})
	}

	if req.Amount <= 0 {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Amount must be positive"})
	}

	islandIDStr := c.QueryParam("island_id")
	if islandIDStr == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Missing island_id query param"})
	}
	islandID, err := uuid.Parse(islandIDStr)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid island_id"})
	}

	db := repository.GetDB()
	var fleet domain.Fleet
	var island domain.Island

	err = db.Transaction(func(tx *gorm.DB) error {
		// Lock fleet and PRELOAD SHIPS for capacity calculation
		if err := tx.Preload("Ships").First(&fleet, "id = ? AND island_id = ?", req.FleetID, islandID).Error; err != nil {
			return err
		}

		if fleet.Cargo == nil {
			fleet.Cargo = make(map[domain.ResourceType]float64)
		}

		// Check Fleet Cargo Balance
		available := fleet.Cargo[req.Resource]
		if available < req.Amount {
			return fmt.Errorf("insufficient fleet cargo: have %.0f, need %.0f", available, req.Amount)
		}

		// Lock Island
		if err := tx.Preload("Player").First(&island, "id = ?", islandID).Error; err != nil {
			return err
		}
		if island.Resources == nil {
			island.Resources = make(map[domain.ResourceType]float64)
		}

		// EXECUTE TRANSFER (Respect Limits)
		deposited := island.Deposit(req.Resource, req.Amount)
		if deposited <= 0 {
			return fmt.Errorf("island storage full for %s", req.Resource)
		}

		fleet.Cargo[req.Resource] -= deposited
		req.Amount = deposited // Update for response message

		// Clamp floating point weirdness
		if fleet.Cargo[req.Resource] < 0.001 {
			delete(fleet.Cargo, req.Resource)
		}

		// SSOT: Compute Capacity and update fields
		fleet.ComputePayload()

		if err := tx.Save(&fleet).Error; err != nil {
			return err
		}
		if err := tx.Save(&island).Error; err != nil {
			return err
		}

		logger.Info("[CARGO] Transfer Fleet/Island",
			"player", island.Player.Username,
			"fleet", fleet.Name,
			"res", req.Resource,
			"amount", req.Amount)

		return nil
	})

	if err != nil {
		return c.JSON(http.StatusConflict, map[string]string{"error": err.Error()})
	}

	resp := CargoTransferResponse{
		FleetCargo:      fleet.Cargo,
		IslandResources: island.Resources,
		CargoCapacity:   fleet.CargoCapacity,
		CargoUsed:       fleet.CargoUsed,
		CargoFree:       fleet.CargoFree,
		Message:         fmt.Sprintf("Transferred %.0f %s to island", req.Amount, req.Resource),
	}
	return c.JSON(http.StatusOK, resp)
}
