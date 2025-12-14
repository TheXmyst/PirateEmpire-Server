package economy

import (
	"fmt"
	"math"

	"github.com/TheXmyst/Sea-Dogs/server/internal/domain"
)

// Crew capacity constants per ship type
const (
	CrewCapSloop     = 60
	CrewCapBrigantine = 120
	CrewCapFrigate   = 200
	CrewCapGalleon   = 300
	CrewCapManowar   = 450
)

// Crew quantity bonus divisors (how much crew = 1% bonus)
const (
	CrewAtkDivisor = 200.0 // 200 crew = +1% ATK
	CrewDefDivisor = 250.0 // 250 crew = +1% DEF
)

// Crew quantity bonus caps (maximum bonus)
const (
	CrewAtkCap = 0.10 // max +10% ATK
	CrewDefCap = 0.10 // max +10% DEF
)

// RPS multipliers (already used in naval_combat.go, but centralized here for reference)
const (
	RPSAdvantage   = 1.15 // Advantage multiplier
	RPSDisadvantage = 0.85 // Disadvantage multiplier
	RPSNeutral     = 1.00 // Neutral multiplier
)

// CrewTotal returns the total crew count for a ship
func CrewTotal(ship *domain.Ship) int {
	return ship.CrewWarriors + ship.CrewArchers + ship.CrewGunners
}

// MaxCrewForShipType returns the maximum crew capacity for a given ship type
func MaxCrewForShipType(shipType string) int {
	switch shipType {
	case "sloop":
		return CrewCapSloop
	case "brigantine":
		return CrewCapBrigantine
	case "frigate":
		return CrewCapFrigate
	case "galleon":
		return CrewCapGalleon
	case "manowar":
		return CrewCapManowar
	default:
		// Default to sloop capacity for unknown types
		return CrewCapSloop
	}
}

// ValidateShipCrewBounds validates that a ship's crew is within acceptable bounds
// Returns: (isValid, reasonCode, reasonMessage)
func ValidateShipCrewBounds(ship *domain.Ship) (bool, string, string) {
	// Check for negative values
	if ship.CrewWarriors < 0 || ship.CrewArchers < 0 || ship.CrewGunners < 0 {
		return false, "CREW_INVALID_NEGATIVE", "Équipage invalide (valeur négative)"
	}

	// Check total crew against cap
	totalCrew := CrewTotal(ship)
	maxCrew := MaxCrewForShipType(ship.Type)
	if totalCrew > maxCrew {
		return false, "CREW_INVALID_OVER_CAP", fmt.Sprintf("Équipage trop élevé pour un %s (%d/%d)", ship.Type, totalCrew, maxCrew)
	}

	// Check for unknown ship type (optional, but helpful)
	if MaxCrewForShipType(ship.Type) == CrewCapSloop && ship.Type != "sloop" {
		// This is a warning, not an error, but we'll allow it
		// Could add a log here if needed
	}

	return true, "", ""
}

// ComputeCrewAtkDefBonus calculates ATK and DEF bonuses based on crew total
// Returns: (atkBonus, defBonus) as multipliers (e.g., 0.01 = +1%)
func ComputeCrewAtkDefBonus(crewTotal int) (float64, float64) {
	// Calculate raw bonuses
	atkBonusRaw := float64(crewTotal) / CrewAtkDivisor * 0.01 // 200 crew = 1% = 0.01
	defBonusRaw := float64(crewTotal) / CrewDefDivisor * 0.01 // 250 crew = 1% = 0.01

	// Clamp to caps
	atkBonus := math.Min(atkBonusRaw, CrewAtkCap)
	defBonus := math.Min(defBonusRaw, CrewDefCap)

	// Ensure non-negative
	if atkBonus < 0 {
		atkBonus = 0
	}
	if defBonus < 0 {
		defBonus = 0
	}

	return atkBonus, defBonus
}

