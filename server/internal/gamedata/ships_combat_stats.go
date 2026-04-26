package gamedata

import (
	"fmt"
)

// ShipBaseStats represents the base combat stats for a ship type
type ShipBaseStats struct {
	HP                float64 // Base HP (from max_health in ships.json)
	Speed             float64 // Base speed (from base_speed in ships.json)
	DamageReduction   float64 // Base damage reduction (0.0 = no reduction, 0.5 = 50% reduction)
	RumConsumption    float64 // Base rum consumption per hour (or per engagement)
}

// GetShipBaseStats returns the base combat stats for a given ship type
// These are placeholder values that can be tuned later
func GetShipBaseStats(shipType string) (ShipBaseStats, error) {
	switch shipType {
	case "sloop":
		return ShipBaseStats{
			HP:              100.0,
			Speed:           1.0,
			DamageReduction: 0.0,  // 0% base reduction
			RumConsumption:  10.0,  // 10 rum per hour/engagement
		}, nil
	case "brigantine":
		return ShipBaseStats{
			HP:              250.0,
			Speed:           0.9,
			DamageReduction: 0.05, // 5% base reduction
			RumConsumption:  20.0,
		}, nil
	case "frigate":
		return ShipBaseStats{
			HP:              500.0,
			Speed:           1.1,
			DamageReduction: 0.10, // 10% base reduction
			RumConsumption:  30.0,
		}, nil
	case "galleon":
		return ShipBaseStats{
			HP:              1200.0,
			Speed:           0.7,
			DamageReduction: 0.15, // 15% base reduction
			RumConsumption:  50.0,
		}, nil
	case "manowar":
		return ShipBaseStats{
			HP:              3000.0,
			Speed:           0.6,
			DamageReduction: 0.20, // 20% base reduction
			RumConsumption:  80.0,
		}, nil
	default:
		return ShipBaseStats{}, fmt.Errorf("unknown ship type: %s", shipType)
	}
}

