package economy

import (
	"github.com/TheXmyst/Sea-Dogs/server/internal/domain"
)

// ComputeFleetPayload calculates the total capacity, used space, and free space of a fleet.
// This is the Single Source of Truth for cargo constraints.
func ComputeFleetPayload(f *domain.Fleet) (capacity, used, free float64) {
	// 1. Calculate Capacity (Sum of Ship Holds)
	capacity = 0.0
	for _, s := range f.Ships {
		cap := 0.0
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
		default:
			cap = 500 // Fallback
		}
		capacity += cap
	}

	// 2. Calculate Used Space
	used = 0.0
	if f.Cargo != nil {
		for _, amount := range f.Cargo {
			used += amount
		}
	}

	// 3. Calculate Free Space
	free = capacity - used
	if free < 0 {
		free = 0
	}

	return capacity, used, free
}

// CalculateRumConsumptionPerMinute returns the amount of Rum consumed per minute by the fleet.
// It considers ship count, ship tiers, and captain bonuses.
func CalculateRumConsumptionPerMinute(f *domain.Fleet, captain *domain.Captain) float64 {
	// Base Formula:
	// 1 Unit/Min per Ship
	// + 0.5 Units/Min per Tier Level (Pseudo-tier based on max HP or hull size)

	baseConsumption := 0.0

	for _, s := range f.Ships {
		hullFactor := 1.0
		switch s.Type {
		case "sloop":
			hullFactor = 1.0 // Tier 1
		case "brigantine":
			hullFactor = 2.0 // Tier 2
		case "frigate":
			hullFactor = 3.0 // Tier 3
		case "galleon":
			hullFactor = 4.0 // Tier 4
		case "manowar":
			hullFactor = 5.0 // Tier 5
		}

		// 1 per ship + 0.5 per tier
		// Sloop: 1 + 0.5 = 1.5/min
		// Manowar: 1 + 2.5 = 3.5/min
		shipCons := 1.0 + (hullFactor * 0.5)
		baseConsumption += shipCons
	}

	// Apply Bonuses (SSOT via CaptainRumConsumptionReductionPct)
	multiplier := 1.0
	if captain != nil {
		reduction := CaptainRumConsumptionReductionPct(captain)
		multiplier -= reduction
	}

	if multiplier < 0.1 {
		multiplier = 0.1
	} // Cap reduction at 90%

	return baseConsumption * multiplier
}
