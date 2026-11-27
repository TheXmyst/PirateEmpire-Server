package economy

import (
	"fmt"
	"math"

	"github.com/TheXmyst/Sea-Dogs/server/internal/domain"
)

// =============================================================================
//  MORAL AU COMBAT  (engagement_morale.go)
// =============================================================================
//
// Au début d'un combat, chaque camp a un « moral d'engagement » :
//
//   moral de croisière (0-100, voir fleet_morale.go)
//   − 20 si la flotte est en mer sans rhum (pas pour les PNJ)
//   − les dégâts au moral infligés par le capitaine adverse
//   puis le plancher de son propre capitaine, puis borné entre 0 et 100.
//
// Le camp le plus motivé gagne un bonus d'attaque ET de défense selon l'écart
// (voir tierBonus). Son capitaine peut amplifier ce bonus, celui de l'adversaire
// peut le réduire.
// =============================================================================

// RumMoralePenalty : moral en moins au combat pour une flotte en mer sans rhum.
const RumMoralePenalty = 20

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
	Applied           []string `json:"applied,omitempty"` // Human-readable debug notes
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
	}
	return 0.40
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

// fleetLacksRum : une flotte de joueur en mer sans rhum dans sa cale.
// Une flotte au port (Idle) a le rhum de son île, et les PNJ n'en ont pas besoin.
func fleetLacksRum(f *domain.Fleet) bool {
	if IsNpcFleet(f) || f.State == domain.FleetStateIdle {
		return false
	}
	return f.Cargo == nil || f.Cargo[domain.Rum] <= 0
}

// ComputeEngagementMorale computes the engagement morale snapshot for two fleets
// captA and captB are the captains of each fleet's flagship (can be nil)
func ComputeEngagementMorale(fleetA, fleetB domain.Fleet, captA, captB *domain.Captain) EngagementResult {
	result := EngagementResult{
		FleetAID: fleetA.ID.String(),
		FleetBID: fleetB.ID.String(),
		Applied:  make([]string, 0),
	}
	effA := ComputeCaptainEffects(captA)
	effB := ComputeCaptainEffects(captB)

	moraleA := FleetMorale(&fleetA)
	moraleB := FleetMorale(&fleetB)
	result.Applied = append(result.Applied, fmt.Sprintf("Base morale A=%d B=%d", moraleA, moraleB))

	// 1. Pénalités : rhum, puis capitaine adverse.
	if fleetLacksRum(&fleetA) {
		moraleA -= RumMoralePenalty
		result.Applied = append(result.Applied, fmt.Sprintf("FleetA: out of rum (-%d)", RumMoralePenalty))
	}
	if fleetLacksRum(&fleetB) {
		moraleB -= RumMoralePenalty
		result.Applied = append(result.Applied, fmt.Sprintf("FleetB: out of rum (-%d)", RumMoralePenalty))
	}
	if effA.EnemyMoraleDamage > 0 {
		moraleB -= effA.EnemyMoraleDamage
		result.Applied = append(result.Applied, fmt.Sprintf("FleetA captain: enemy morale -%d", effA.EnemyMoraleDamage))
	}
	if effB.EnemyMoraleDamage > 0 {
		moraleA -= effB.EnemyMoraleDamage
		result.Applied = append(result.Applied, fmt.Sprintf("FleetB captain: enemy morale -%d", effB.EnemyMoraleDamage))
	}

	// 2. Planchers (après toutes les pénalités, pour les deux camps pareil).
	if effA.MoraleFloor > 0 && moraleA < effA.MoraleFloor {
		result.Applied = append(result.Applied, fmt.Sprintf("FleetA captain: floor %d -> %d", moraleA, effA.MoraleFloor))
		moraleA = effA.MoraleFloor
	}
	if effB.MoraleFloor > 0 && moraleB < effB.MoraleFloor {
		result.Applied = append(result.Applied, fmt.Sprintf("FleetB captain: floor %d -> %d", moraleB, effB.MoraleFloor))
		moraleB = effB.MoraleFloor
	}

	moraleA = clampMorale(moraleA)
	moraleB = clampMorale(moraleB)
	result.EngagementMoraleA = moraleA
	result.EngagementMoraleB = moraleB

	delta := moraleA - moraleB
	result.Delta = delta
	absDelta := int(math.Abs(float64(delta)))
	bonus := tierBonus(absDelta)

	// 3. Capitaines qui modifient le bonus du gagnant.
	if delta > 0 {
		bonus *= (1 + effA.MoraleBonusAmplify) * (1 - effB.EnemyMoraleBonusReduction)
	} else if delta < 0 {
		bonus *= (1 + effB.MoraleBonusAmplify) * (1 - effA.EnemyMoraleBonusReduction)
	}
	if bonus > maxEngagementBonusPct {
		bonus = maxEngagementBonusPct
	}
	result.BonusPercent = bonus
	result.Applied = append(result.Applied, fmt.Sprintf("Delta: %d (abs=%d) => +%.1f%%", delta, absDelta, bonus*100))

	result.AtkMultA, result.DefMultA, result.AtkMultB, result.DefMultB = 1, 1, 1, 1
	if delta > 0 {
		result.AtkMultA = 1 + bonus
		result.DefMultA = 1 + bonus
	} else if delta < 0 {
		result.AtkMultB = 1 + bonus
		result.DefMultB = 1 + bonus
	}
	return result
}
