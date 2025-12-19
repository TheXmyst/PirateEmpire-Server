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

		// Calculate Fleet Max Capacity
		capacity := calculateFleetCapacity(&fleet)

		// Calculate Current Load
		if fleet.Cargo == nil {
			fleet.Cargo = make(map[domain.ResourceType]float64)
		}
		currentLoad := 0.0
		for _, v := range fleet.Cargo {
			currentLoad += v
		}

		// Check Capacity
		if currentLoad+req.Amount > capacity {
			return fmt.Errorf("insufficient fleet capacity: available %.0f, need %.0f", capacity-currentLoad, req.Amount)
		}

		// EXECUTE TRANSFER
		island.Resources[req.Resource] -= req.Amount
		fleet.Cargo[req.Resource] += req.Amount

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
		// Lock fleet
		if err := tx.First(&fleet, "id = ? AND island_id = ?", req.FleetID, islandID).Error; err != nil {
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

		// EXECUTE TRANSFER
		fleet.Cargo[req.Resource] -= req.Amount
		island.Resources[req.Resource] += req.Amount

		// Clamp floating point weirdness
		if fleet.Cargo[req.Resource] < 0.001 {
			delete(fleet.Cargo, req.Resource)
		}

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
		Message:         fmt.Sprintf("Transferred %.0f %s to island", req.Amount, req.Resource),
	}
	return c.JSON(http.StatusOK, resp)
}

// Helper to calculate capacity (duplicated logic, should be centralized ideally but OK for now)
func calculateFleetCapacity(f *domain.Fleet) float64 {
	capacity := 0.0
	for _, s := range f.Ships {
		cap := 500.0
		switch s.Type {
		case "sloop":
			cap = 500
		case "brigantine":
			cap = 1500
		case "frigate":
			cap = 3000
		case "galleon":
			cap = 8000
		case "manowar":
			cap = 12000
		}
		capacity += cap
	}
	return capacity
}
