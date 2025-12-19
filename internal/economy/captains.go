package economy

import (
	"fmt"
	"math"

	"github.com/TheXmyst/Sea-Dogs/server/internal/domain"
)

// CaptainPassiveEffect represents the computed passive effect of a captain
type CaptainPassiveEffect struct {
	ID             string          `json:"passive_id,omitempty"`
	Value          float64         `json:"passive_value,omitempty"`
	IntValue       int             `json:"passive_int_value,omitempty"`
	Threshold      int             `json:"threshold,omitempty"`
	DrainPerMinute float64         `json:"drain_per_minute,omitempty"`
	Flags          map[string]bool `json:"flags,omitempty"`
}

// ClampCaptainLevel ensures level is between 1 and 80
func ClampCaptainLevel(level int) int {
	if level < 1 {
		return 1
	}
	if level > 80 {
		return 80
	}
	return level
}

// ScaleFloat scales a float value from base to max based on level (1-80)
// Formula: value = base + (level-1)/(80-1) * (max-base)
func ScaleFloat(level int, base, max float64) float64 {
	clamped := ClampCaptainLevel(level)
	if clamped == 1 {
		return base
	}
	if clamped == 80 {
		return max
	}
	progress := float64(clamped-1) / 79.0 // 80-1 = 79
	return base + progress*(max-base)
}

// ScaleInt scales an int value from base to max based on level (1-80)
func ScaleInt(level int, base, max int) int {
	clamped := ClampCaptainLevel(level)
	if clamped == 1 {
		return base
	}
	if clamped == 80 {
		return max
	}
	progress := float64(clamped-1) / 79.0
	return base + int(math.Round(progress*float64(max-base)))
}

// ComputeCaptainPassive computes the passive effect for a captain based on SkillID and Level
func ComputeCaptainPassive(c domain.Captain) CaptainPassiveEffect {
	effect := CaptainPassiveEffect{
		ID:    c.SkillID,
		Flags: make(map[string]bool),
	}

	level := ClampCaptainLevel(c.Level)

	switch c.SkillID {
	// COMMON PASSIVES
	case "nav_morale_decay_reduction":
		effect.Value = ScaleFloat(level, 0.02, 0.10)
	case "rum_consumption_reduction":
		effect.Value = ScaleFloat(level, 0.02, 0.10)
	case "morale_floor":
		effect.IntValue = ScaleInt(level, 8, 20)
	case "wind_favorable_speed_bonus":
		effect.Value = ScaleFloat(level, 0.02, 0.08)
	case "port_morale_recovery_bonus":
		effect.Value = ScaleFloat(level, 0.08, 0.25)
	case "crew_loss_reduction":
		effect.Value = ScaleFloat(level, 0.02, 0.10)
	case "wind_unfavorable_penalty_reduction":
		effect.Value = ScaleFloat(level, 0.05, 0.20)
	case "low_morale_decay_slowdown":
		effect.Value = ScaleFloat(level, 0.05, 0.15)

	// RARE PASSIVES
	case "interception_chance_bonus":
		effect.Value = ScaleFloat(level, 0.06, 0.25)
	case "opening_enemy_morale_damage":
		effect.IntValue = ScaleInt(level, 6, 25)
	case "enemy_morale_decay_multiplier":
		effect.Value = ScaleFloat(level, 0.15, 0.50)
	case "low_morale_speed_bonus":
		effect.Value = ScaleFloat(level, 0.06, 0.22)
		effect.Threshold = ScaleInt(level, 35, 50)
	case "panic_immunity_threshold":
		effect.Threshold = ScaleInt(level, 35, 60)

	// LEGENDARY PASSIVES
	case "wind_never_unfavorable":
		effect.Flags["wind_never_unfavorable"] = true
		if level == 80 {
			effect.Value = 0.10 // +10% speed bonus at max level
		}
	case "terror_engagement":
		effect.IntValue = ScaleInt(level, 10, 30)
		effect.DrainPerMinute = ScaleFloat(level, 0.5, 2.0)
	case "absolute_morale_floor":
		effect.IntValue = ScaleInt(level, 25, 60)

	default:
		// Unknown skill ID - return empty effect
		fmt.Printf("[CAPTAIN] ComputeCaptainPassive: Unknown skill_id '%s' for captain %s\n", c.SkillID, c.ID)
		return effect
	}

	return effect
}

// CalculateCaptainSpeedMultiplier returns the speed multiplier based on captain skills
func CalculateCaptainSpeedMultiplier(c domain.Captain, isFavorableWind bool) float64 {
	multiplier := 1.0
	passive := ComputeCaptainPassive(c)

	switch c.SkillID {
	case "wind_favorable_speed_bonus":
		if isFavorableWind {
			multiplier += passive.Value
		}
	case "wind_never_unfavorable":
		if isFavorableWind {
			multiplier += passive.Value // +10% at max level as per ComputeCaptainPassive
		}
	case "low_morale_speed_bonus":
		// For NPC fleets, we could assume they are always at good morale
		// or random. V1 simplification: Skip morale-based speed in random walk.
	}

	return multiplier
}
