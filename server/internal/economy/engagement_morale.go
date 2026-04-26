package economy

import (
	"fmt"
	"math"

	"github.com/TheXmyst/Sea-Dogs/server/internal/domain"
)

// EngagementResult represents the computed engagement morale snapshot and multipliers
type EngagementResult struct {
	FleetAID          string   `json:"fleet_a_id"`
	FleetBID          string   `json:"fleet_b_id"`
	EngagementMoraleA int      `json:"engagement_morale_a"`
	EngagementMoraleB int      `json:"engagement_morale_b"`
	Delta             int      `json:"delta"`
	BonusPercent      float64  `json:"bonus_percent"`
	AtkMultA          float64  `json:"atk_mult_a"`
	DefMultA          float64  `json:"def_mult_a"`
	AtkMultB          float64  `json:"atk_mult_b"`
	DefMultB          float64  `json:"def_mult_b"`
	PanicThresholdA   int      `json:"panic_threshold_a,omitempty"` // Future use: panic immunity threshold
	PanicThresholdB   int      `json:"panic_threshold_b,omitempty"` // Future use: panic immunity threshold
	Applied           []string `json:"applied,omitempty"`           // Human-readable debug notes
}

// tierBonus returns the bonus percentage based on absolute delta using punitive tier table
// abs(dM) in:
//
//	0-4   -> 0%
//	5-9   -> 5%
//	10-19 -> 10%
//	20-29 -> 18%
//	30-39 -> 28%
//	40+   -> 40% (hard cap)
func tierBonus(absDelta int) float64 {
	if absDelta <= 4 {
		return 0.0
	} else if absDelta <= 9 {
		return 0.05
	} else if absDelta <= 19 {
		return 0.10
	} else if absDelta <= 29 {
		return 0.18
	} else if absDelta <= 39 {
		return 0.28
	} else {
		return 0.40 // Hard cap at 40%
	}
}

// clampMorale ensures morale is between 0 and 100
func clampMorale(morale int) int {
	if morale < 0 {
		return 0
	}
	if morale > 100 {
		return 100
	}
	return morale
}

// ComputeEngagementMorale computes the engagement morale snapshot for two fleets
// captA and captB are the captains assigned to the flagship of each fleet (can be nil)
func ComputeEngagementMorale(fleetA, fleetB domain.Fleet, captA, captB *domain.Captain) EngagementResult {
	result := EngagementResult{
		FleetAID: fleetA.ID.String(),
		FleetBID: fleetB.ID.String(),
		Applied:  make([]string, 0),
	}

	// Start with cruise morale (default 50 if NULL/uninitialized, otherwise use actual value)
	var moraleA int
	if fleetA.MoraleCruise == nil {
		moraleA = 50
		result.Applied = append(result.Applied, "FleetA: morale_cruise=NULL -> default 50")
	} else {
		moraleA = *fleetA.MoraleCruise
		result.Applied = append(result.Applied, fmt.Sprintf("FleetA: base morale_cruise=%d", moraleA))
	}

	var moraleB int
	if fleetB.MoraleCruise == nil {
		moraleB = 50
		result.Applied = append(result.Applied, "FleetB: morale_cruise=NULL -> default 50")
	} else {
		moraleB = *fleetB.MoraleCruise
		result.Applied = append(result.Applied, fmt.Sprintf("FleetB: base morale_cruise=%d", moraleB))
	}

	// RUM PENALTY (-20 if Out of Rum)
	// Phase D: Combat Morale Penalty
	if fleetA.Cargo == nil || fleetA.Cargo[domain.Rum] <= 0 {
		moraleA -= 20
		result.Applied = append(result.Applied, "FleetA: Out of Rum (-20)")
	}
	if fleetB.Cargo == nil || fleetB.Cargo[domain.Rum] <= 0 {
		moraleB -= 20
		result.Applied = append(result.Applied, "FleetB: Out of Rum (-20)")
	}

	// Apply captain engagement effects
	// LEGENDARY: absolute_morale_floor
	if captA != nil {
		effectA := ComputeCaptainPassive(*captA)
		beforeA := moraleA
		if effectA.ID == "absolute_morale_floor" {
			floor := effectA.IntValue
			if moraleA < floor {
				moraleA = floor
				result.Applied = append(result.Applied, fmt.Sprintf("FleetA captain: absolute_morale_floor %d -> %d", beforeA, moraleA))
			}
		}
		// RARE: opening_enemy_morale_damage and terror_engagement reduce enemy morale
		beforeB := moraleB
		if effectA.ID == "opening_enemy_morale_damage" {
			damage := effectA.IntValue
			moraleB -= damage
			result.Applied = append(result.Applied, fmt.Sprintf("FleetA captain: opening_enemy_morale_damage FleetB %d -> %d (-%d)", beforeB, moraleB, damage))
		} else if effectA.ID == "terror_engagement" {
			damage := effectA.IntValue
			moraleB -= damage
			result.Applied = append(result.Applied, fmt.Sprintf("FleetA captain: terror_engagement FleetB %d -> %d (-%d)", beforeB, moraleB, damage))
		}
		// RARE: panic_immunity_threshold (expose for future use, no gameplay effect yet)
		if effectA.ID == "panic_immunity_threshold" {
			result.PanicThresholdA = effectA.Threshold
			result.Applied = append(result.Applied, fmt.Sprintf("FleetA captain: panic_immunity_threshold=%d (exposed, not applied)", effectA.Threshold))
		}
	}

	if captB != nil {
		effectB := ComputeCaptainPassive(*captB)
		beforeB := moraleB
		if effectB.ID == "absolute_morale_floor" {
			floor := effectB.IntValue
			if moraleB < floor {
				moraleB = floor
				result.Applied = append(result.Applied, fmt.Sprintf("FleetB captain: absolute_morale_floor %d -> %d", beforeB, moraleB))
			}
		}
		// RARE: opening_enemy_morale_damage and terror_engagement reduce enemy morale
		beforeA := moraleA
		if effectB.ID == "opening_enemy_morale_damage" {
			damage := effectB.IntValue
			moraleA -= damage
			result.Applied = append(result.Applied, fmt.Sprintf("FleetB captain: opening_enemy_morale_damage FleetA %d -> %d (-%d)", beforeA, moraleA, damage))
		} else if effectB.ID == "terror_engagement" {
			damage := effectB.IntValue
			moraleA -= damage
			result.Applied = append(result.Applied, fmt.Sprintf("FleetB captain: terror_engagement FleetA %d -> %d (-%d)", beforeA, moraleA, damage))
		}
		// RARE: panic_immunity_threshold (expose for future use, no gameplay effect yet)
		if effectB.ID == "panic_immunity_threshold" {
			result.PanicThresholdB = effectB.Threshold
			result.Applied = append(result.Applied, fmt.Sprintf("FleetB captain: panic_immunity_threshold=%d (exposed, not applied)", effectB.Threshold))
		}
	}

	// Clamp morale to 0-100
	beforeClampA := moraleA
	beforeClampB := moraleB
	moraleA = clampMorale(moraleA)
	moraleB = clampMorale(moraleB)
	if beforeClampA != moraleA {
		result.Applied = append(result.Applied, fmt.Sprintf("FleetA: clamped %d -> %d", beforeClampA, moraleA))
	}
	if beforeClampB != moraleB {
		result.Applied = append(result.Applied, fmt.Sprintf("FleetB: clamped %d -> %d", beforeClampB, moraleB))
	}

	result.EngagementMoraleA = moraleA
	result.EngagementMoraleB = moraleB

	// Compute delta
	delta := moraleA - moraleB
	result.Delta = delta

	// Compute tier bonus based on absolute delta
	absDelta := int(math.Abs(float64(delta)))
	bonusPercent := tierBonus(absDelta)
	result.BonusPercent = bonusPercent

	// Determine tier description
	var tierDesc string
	if absDelta <= 4 {
		tierDesc = "0-4"
	} else if absDelta <= 9 {
		tierDesc = "5-9"
	} else if absDelta <= 19 {
		tierDesc = "10-19"
	} else if absDelta <= 29 {
		tierDesc = "20-29"
	} else if absDelta <= 39 {
		tierDesc = "30-39"
	} else {
		tierDesc = "40+"
	}
	result.Applied = append(result.Applied, fmt.Sprintf("Delta: %d (absΔ=%d => tier %s => +%.0f%%)", delta, absDelta, tierDesc, bonusPercent*100))

	// Determine winner and apply multipliers
	if delta > 0 {
		// FleetA wins
		result.AtkMultA = 1.0 + bonusPercent
		result.DefMultA = 1.0 + bonusPercent
		result.AtkMultB = 1.0
		result.DefMultB = 1.0
		result.Applied = append(result.Applied, fmt.Sprintf("FleetA wins: atk/def mult=%.2f", result.AtkMultA))
	} else if delta < 0 {
		// FleetB wins
		result.AtkMultA = 1.0
		result.DefMultA = 1.0
		result.AtkMultB = 1.0 + bonusPercent
		result.DefMultB = 1.0 + bonusPercent
		result.Applied = append(result.Applied, fmt.Sprintf("FleetB wins: atk/def mult=%.2f", result.AtkMultB))
	} else {
		// Tie
		result.AtkMultA = 1.0
		result.DefMultA = 1.0
		result.AtkMultB = 1.0
		result.DefMultB = 1.0
		result.Applied = append(result.Applied, "Tie: no bonus (mult=1.0)")
	}

	return result
}
