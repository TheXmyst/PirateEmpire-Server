package economy

import (
	"fmt"
	"math"

	"github.com/TheXmyst/Sea-Dogs/server/internal/domain"
	"github.com/TheXmyst/Sea-Dogs/server/internal/gamedata"
)

// ShipCombatStats represents the computed combat stats for a ship with captain bonuses applied
type ShipCombatStats struct {
	BaseHP                    float64  `json:"base_hp"`
	EffectiveHP              float64  `json:"effective_hp"`
	BaseSpeed                float64  `json:"base_speed"`
	EffectiveSpeed           float64  `json:"effective_speed"`
	BaseDamageReduction      float64  `json:"base_damage_reduction"`
	EffectiveDamageReduction float64  `json:"effective_damage_reduction"`
	BaseRumConsumption       float64  `json:"base_rum_consumption"`
	EffectiveRumConsumption  float64  `json:"effective_rum_consumption"`
	Applied                  []string `json:"applied,omitempty"` // Human-readable debug notes
}

// ComputeShipCombatStatsWithCaptain computes the effective combat stats for a ship with captain star bonuses
// If captain is nil or has 0 stars, effective stats equal base stats
// This function is called at engagement time only, not persisted to DB
func ComputeShipCombatStatsWithCaptain(ship *domain.Ship, captain *domain.Captain) (ShipCombatStats, error) {
	// Get base stats from ship type
	baseStats, err := gamedata.GetShipBaseStats(ship.Type)
	if err != nil {
		return ShipCombatStats{}, fmt.Errorf("failed to get base stats for ship type %s: %w", ship.Type, err)
	}

	result := ShipCombatStats{
		BaseHP:                baseStats.HP,
		BaseSpeed:             baseStats.Speed,
		BaseDamageReduction:   baseStats.DamageReduction,
		BaseRumConsumption:    baseStats.RumConsumption,
		EffectiveHP:           baseStats.HP,
		EffectiveSpeed:        baseStats.Speed,
		EffectiveDamageReduction: baseStats.DamageReduction,
		EffectiveRumConsumption:  baseStats.RumConsumption,
		Applied:               make([]string, 0),
	}

	// If no captain or captain has 0 stars, return base stats
	if captain == nil || captain.Stars == 0 {
		result.Applied = append(result.Applied, "no captain stars bonus")
		return result, nil
	}

	// Compute naval bonuses from captain stars (reuse existing function)
	navalBonuses := ComputeNavalBonuses(*captain)

	// Apply HP bonus (multiplicative: EffectiveHP = BaseHP * (1 + hpBonus))
	if navalBonuses.NavalHPBonusPct > 0 {
		before := result.EffectiveHP
		result.EffectiveHP = baseStats.HP * (1.0 + navalBonuses.NavalHPBonusPct)
		result.Applied = append(result.Applied, fmt.Sprintf("HP: %.1f -> %.1f (+%.1f%%)", before, result.EffectiveHP, navalBonuses.NavalHPBonusPct*100))
	}

	// Apply Speed bonus (multiplicative: EffectiveSpeed = BaseSpeed * (1 + speedBonus))
	if navalBonuses.NavalSpeedBonusPct > 0 {
		before := result.EffectiveSpeed
		result.EffectiveSpeed = baseStats.Speed * (1.0 + navalBonuses.NavalSpeedBonusPct)
		result.Applied = append(result.Applied, fmt.Sprintf("Speed: %.2f -> %.2f (+%.1f%%)", before, result.EffectiveSpeed, navalBonuses.NavalSpeedBonusPct*100))
	}

	// Apply Damage Reduction bonus (additive but clamped to 0.90 max)
	if navalBonuses.NavalDamageReductionPct > 0 {
		before := result.EffectiveDamageReduction
		result.EffectiveDamageReduction = baseStats.DamageReduction + navalBonuses.NavalDamageReductionPct
		// Clamp to 90% max
		if result.EffectiveDamageReduction > 0.90 {
			result.EffectiveDamageReduction = 0.90
		}
		// Clamp to 0 min
		if result.EffectiveDamageReduction < 0.0 {
			result.EffectiveDamageReduction = 0.0
		}
		result.Applied = append(result.Applied, fmt.Sprintf("DamageReduction: %.2f%% -> %.2f%% (+%.1f%%)", before*100, result.EffectiveDamageReduction*100, navalBonuses.NavalDamageReductionPct*100))
	}

	// Apply Rum Consumption reduction (multiplicative: EffectiveRum = BaseRum * (1 - rumReduction))
	if navalBonuses.RumConsumptionReductionPct > 0 {
		before := result.EffectiveRumConsumption
		result.EffectiveRumConsumption = baseStats.RumConsumption * (1.0 - navalBonuses.RumConsumptionReductionPct)
		// Clamp to minimum 0.1 (10% of base) to avoid zero consumption
		if result.EffectiveRumConsumption < 0.1 {
			result.EffectiveRumConsumption = math.Max(0.1, baseStats.RumConsumption*0.1)
		}
		result.Applied = append(result.Applied, fmt.Sprintf("RumConsumption: %.1f -> %.1f (-%.1f%%)", before, result.EffectiveRumConsumption, navalBonuses.RumConsumptionReductionPct*100))
	}

	// Add summary note
	if len(result.Applied) > 0 {
		result.Applied = append([]string{fmt.Sprintf("Captain: %s (Rarity: %s, Stars: %d)", captain.Name, captain.Rarity, captain.Stars)}, result.Applied...)
	}

	return result, nil
}

