package economy

import (
	"fmt"

	"github.com/TheXmyst/Sea-Dogs/server/internal/domain"
)

// Max stars per rarity (0..MaxStars)
const (
	MaxStarsCommon    = 5
	MaxStarsRare      = 5
	MaxStarsLegendary = 5
)

// Hard cap for naval bonuses (safety limit)
const (
	MaxNavalHPBonusPct              = 0.10 // 10%
	MaxNavalSpeedBonusPct           = 0.10 // 10%
	MaxNavalDamageReductionPct       = 0.10 // 10%
	MaxRumConsumptionReductionPct   = 0.10 // 10%
)

// GetMaxStars returns the maximum stars for a given rarity
func GetMaxStars(rarity domain.CaptainRarity) int {
	switch rarity {
	case domain.RarityCommon:
		return MaxStarsCommon
	case domain.RarityRare:
		return MaxStarsRare
	case domain.RarityLegendary:
		return MaxStarsLegendary
	default:
		return 0
	}
}

// GetStarUpgradeCost returns the shard cost to upgrade from currentStars to currentStars+1
// Stars is 0-based: 0 = no stars, MaxStars = fully upgraded
// "max reached" means currentStars == MaxStars
func GetStarUpgradeCost(rarity domain.CaptainRarity, currentStars int) (int, error) {
	maxStars := GetMaxStars(rarity)
	if currentStars >= maxStars {
		return 0, fmt.Errorf("captain already at max stars (%d)", maxStars)
	}
	if currentStars < 0 {
		return 0, fmt.Errorf("invalid star count: %d", currentStars)
	}

	// Target star level (1-based for cost calculation)
	targetStar := currentStars + 1
	// Base cost per rarity
	var base int
	switch rarity {
	case domain.RarityCommon:
		base = 10
	case domain.RarityRare:
		base = 20
	case domain.RarityLegendary:
		base = 40
	default:
		return 0, fmt.Errorf("unknown rarity: %s", rarity)
	}

	// Cost scales linearly by target star (1-based)
	return base * targetStar, nil
}

// NavalBonusDTO represents computed naval bonuses from stars
type NavalBonusDTO struct {
	NavalHPBonusPct            float64 `json:"naval_hp_bonus_pct"`
	NavalSpeedBonusPct         float64 `json:"naval_speed_bonus_pct"`
	NavalDamageReductionPct    float64 `json:"naval_damage_reduction_pct"`
	RumConsumptionReductionPct float64 `json:"rum_consumption_reduction_pct"`
}

// ComputeNavalBonuses calculates naval bonuses from captain stars
// Fixed per-star increments (linear scaling)
// Stars value represents "number of stars acquired" (0-based, clamped to [0..MaxStars])
func ComputeNavalBonuses(captain domain.Captain) NavalBonusDTO {
	stars := captain.Stars
	if stars < 0 {
		stars = 0
	}
	maxStars := GetMaxStars(captain.Rarity)
	if stars > maxStars {
		stars = maxStars
	}

	var bonus NavalBonusDTO

	// Fixed per-star increments (no progress-based scaling)
	switch captain.Rarity {
	case domain.RarityCommon:
		// Common: +1.0% HP, +0.5% Speed, +0.5% DR, -0.5% Rum per star
		bonus.NavalHPBonusPct = float64(stars) * 0.01            // 1.0% per star
		bonus.NavalSpeedBonusPct = float64(stars) * 0.005         // 0.5% per star
		bonus.NavalDamageReductionPct = float64(stars) * 0.005   // 0.5% per star
		bonus.RumConsumptionReductionPct = float64(stars) * 0.005 // 0.5% per star (reduction)
	case domain.RarityRare:
		// Rare: +1.5% HP, +0.75% Speed, +0.75% DR, -0.75% Rum per star
		bonus.NavalHPBonusPct = float64(stars) * 0.015           // 1.5% per star
		bonus.NavalSpeedBonusPct = float64(stars) * 0.0075       // 0.75% per star
		bonus.NavalDamageReductionPct = float64(stars) * 0.0075  // 0.75% per star
		bonus.RumConsumptionReductionPct = float64(stars) * 0.0075 // 0.75% per star (reduction)
	case domain.RarityLegendary:
		// Legendary: +2.0% HP, +1.0% Speed, +1.0% DR, -1.0% Rum per star
		bonus.NavalHPBonusPct = float64(stars) * 0.02            // 2.0% per star
		bonus.NavalSpeedBonusPct = float64(stars) * 0.01        // 1.0% per star
		bonus.NavalDamageReductionPct = float64(stars) * 0.01   // 1.0% per star
		bonus.RumConsumptionReductionPct = float64(stars) * 0.01 // 1.0% per star (reduction)
	default:
		// Unknown rarity: no bonuses
		return NavalBonusDTO{}
	}

	// Apply hard caps (safety)
	if bonus.NavalHPBonusPct > MaxNavalHPBonusPct {
		bonus.NavalHPBonusPct = MaxNavalHPBonusPct
	}
	if bonus.NavalSpeedBonusPct > MaxNavalSpeedBonusPct {
		bonus.NavalSpeedBonusPct = MaxNavalSpeedBonusPct
	}
	if bonus.NavalDamageReductionPct > MaxNavalDamageReductionPct {
		bonus.NavalDamageReductionPct = MaxNavalDamageReductionPct
	}
	if bonus.RumConsumptionReductionPct > MaxRumConsumptionReductionPct {
		bonus.RumConsumptionReductionPct = MaxRumConsumptionReductionPct
	}

	return bonus
}

