package economy

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/TheXmyst/Sea-Dogs/server/internal/domain"
)

type ShipConfig struct {
	Name                  string                          `json:"name"`
	MaxHealth             float64                         `json:"max_health"`
	CargoCapacity         float64                         `json:"cargo_capacity"`
	BaseSpeed             float64                         `json:"base_speed"`
	BuildTime             int                             `json:"build_time"` // Seconds
	Cost                  map[domain.ResourceType]float64 `json:"cost"`
	RepairCostFactor      map[domain.ResourceType]float64 `json:"repair_cost_factor"`
	RequiredShipyardLevel int                             `json:"required_shipyard_level"`
	RequiredTechID        string                          `json:"required_tech"`
}

var ShipInfos map[string]ShipConfig

func LoadShipConfig(path string) error {
	file, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	var data map[string]map[string]ShipConfig
	if err := json.Unmarshal(file, &data); err != nil {
		return err
	}

	if ships, ok := data["ships"]; ok {
		ShipInfos = ships
	} else {
		return fmt.Errorf("invalid ship config format")
	}
	return nil
}

func GetShipStats(shipType string) (ShipConfig, error) {
	if config, ok := ShipInfos[shipType]; ok {
		return config, nil
	}
	return ShipConfig{}, fmt.Errorf("ship type not found: %s", shipType)
}

func CalculateShipBuildTime(shipType string, mods TechModifiers) int {
	config, err := GetShipStats(shipType)
	if err != nil {
		return 3600 // Fallback
	}

	// Apply Build Reduction (Logistics)
	reduction := mods.BuildTimeReduction
	if reduction > 0.9 {
		reduction = 0.9
	}

	finalTime := float64(config.BuildTime) * (1.0 - reduction)
	return int(finalTime)
}

// GetMaxFleets returns the maximum number of fleets a player can have
// based on their Shipyard level.
// Levels 1-9: 1 Fleet
// Levels 10-19: 2 Fleets
// Levels 20+: 3 Fleets
func GetMaxFleets(shipyardLevel int) int {
	if shipyardLevel >= 20 {
		return 3
	}
	if shipyardLevel >= 10 {
		return 2
	}
	return 1
}

// GetMaxShipsPerFleet returns the maximum number of ships a single fleet can hold
// based on unlocked technologies.
func GetMaxShipsPerFleet(mods TechModifiers) int {
	capacity := 3 + mods.FleetSizeBonus
	return capacity
}

// CalculateFleetSpeed calculates the effective speed of a fleet based on its ship composition.
// Returns a weighted average speed multiplier that should be applied to the global base speed (5.0).
// Formula: fleetSpeed = globalBase * (sum(shipCount * shipTypeMultiplier) / totalShips)
func CalculateFleetSpeed(fleet *domain.Fleet) float64 {
	if fleet == nil || len(fleet.Ships) == 0 {
		return 5.0 // Default fallback
	}

	totalWeightedSpeed := 0.0
	totalShips := 0

	for _, ship := range fleet.Ships {
		config, err := GetShipStats(ship.Type)
		if err != nil {
			// Fallback to 1.0 multiplier if ship type not found
			totalWeightedSpeed += 1.0
		} else {
			totalWeightedSpeed += config.BaseSpeed
		}
		totalShips++
	}

	if totalShips == 0 {
		return 5.0
	}

	// Calculate average multiplier
	avgMultiplier := totalWeightedSpeed / float64(totalShips)

	// Apply to global base speed
	return 5.0 * avgMultiplier
}
