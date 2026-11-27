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

// ComputeShipCombatStatsWithCaptain computes the effective combat stats for a ship with its fleet captain's effects
// (talent + stars). If captain is nil, effective stats equal base stats
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

	if captain == nil {
		result.Applied = append(result.Applied, "no captain")
		return result, nil
	}

	// Effets du capitaine de la flotte (talent + étoiles), pour chaque navire.
	e := ComputeCaptainEffects(captain)
	if e.HPPct > 0 {
		result.EffectiveHP = baseStats.HP * (1.0 + e.HPPct)
		result.Applied = append(result.Applied, fmt.Sprintf("HP +%.1f%%", e.HPPct*100))
	}
	if e.SpeedPct > 0 {
		result.EffectiveSpeed = baseStats.Speed * (1.0 + e.SpeedPct)
		result.Applied = append(result.Applied, fmt.Sprintf("Speed +%.1f%%", e.SpeedPct*100))
	}
	if e.DamageTakenReduction > 0 {
		result.EffectiveDamageReduction = math.Min(0.90, math.Max(0, baseStats.DamageReduction+e.DamageTakenReduction))
		result.Applied = append(result.Applied, fmt.Sprintf("DamageReduction +%.1f%%", e.DamageTakenReduction*100))
	}
	if e.RumReductionPct > 0 {
		result.EffectiveRumConsumption = math.Max(baseStats.RumConsumption*0.1, baseStats.RumConsumption*(1.0-e.RumReductionPct))
	}
	result.Applied = append([]string{fmt.Sprintf("Captain: %s (Rarity: %s, Lvl: %d, Stars: %d)", captain.Name, captain.Rarity, captain.Level, captain.Stars)}, result.Applied...)
	return result, nil
}

