package economy

import (
	"math"
	"testing"

	"github.com/TheXmyst/Sea-Dogs/server/internal/domain"
)

// TestPityLogic verifies that the Pity system correctly guarantees rarity
func TestPityLogic(t *testing.T) {
	// 1. Guaranteed Legendary at 80 pity
	rarity, forced := RollCaptainRarityWithPity(80, 0)
	if rarity != domain.RarityLegendary {
		t.Errorf("Expected Legendary at pity 80, got %s", rarity)
	}
	if !forced {
		t.Errorf("Expected forced Legendary at pity 80, got natural")
	}

	// 2. Guaranteed Rare at 10 pity
	// Note: Legendary check comes first in the function, so if (0, 10) -> Rare
	rarity, forced = RollCaptainRarityWithPity(0, 10)
	if rarity != domain.RarityRare && rarity != domain.RarityLegendary {
		// It could be Legendary naturally (low chance), but definitely at least Rare
		t.Errorf("Expected minimal Rare at pity 10, got %s", rarity)
	}
	if !forced && rarity == domain.RarityRare {
		t.Errorf("Expected forced Rare at pity 10, got natural")
	}

	// 3. Normal roll (0 pity) should be random (mostly Common)
	// We can't deterministic test random, but we verify it doesn't panic
	rarity, _ = RollCaptainRarityWithPity(0, 0)
	if rarity == "" {
		t.Errorf("Rolled empty rarity")
	}
}

// TestStarUpgradeCosts verifies the cost progression
func TestStarUpgradeCosts(t *testing.T) {
	tests := []struct {
		rarity       domain.CaptainRarity
		currentStars int
		expectedCost int
		expectError  bool
	}{
		// Common
		{domain.RarityCommon, 0, 10, false},
		{domain.RarityCommon, 1, 20, false},
		{domain.RarityCommon, 2, 30, false},
		{domain.RarityCommon, 3, 40, false},
		{domain.RarityCommon, 4, 50, false},
		{domain.RarityCommon, 5, 0, true}, // Max stars

		// Rare
		{domain.RarityRare, 0, 20, false},
		{domain.RarityRare, 1, 40, false},
		{domain.RarityRare, 4, 100, false},

		// Legendary
		{domain.RarityLegendary, 0, 40, false},
		{domain.RarityLegendary, 1, 80, false},
		{domain.RarityLegendary, 4, 200, false},
	}

	for _, tt := range tests {
		cost, err := GetStarUpgradeCost(tt.rarity, tt.currentStars)
		if tt.expectError {
			if err == nil {
				t.Errorf("Expected error for %s at star %d, got cost %d", tt.rarity, tt.currentStars, cost)
			}
		} else {
			if err != nil {
				t.Errorf("Unexpected error for %s at star %d: %v", tt.rarity, tt.currentStars, err)
			}
			if cost != tt.expectedCost {
				t.Errorf("Wrong cost for %s at star %d: got %d, expected %d", tt.rarity, tt.currentStars, cost, tt.expectedCost)
			}
		}
	}
}

// TestComputeNavalBonuses verifies the SSOT bonus calculations
func TestComputeNavalBonuses(t *testing.T) {
	// 1. Legendary 5 Stars
	// HP: 2.0% * 5 = 10%
	// Speed: 1.0% * 5 = 5%
	// DR: 1.0% * 5 = 5%
	// Rum: 1.0% * 5 = 5%
	capLegendary := domain.Captain{
		Rarity: domain.RarityLegendary,
		Stars:  5,
	}
	bonuses := ComputeNavalBonuses(capLegendary)

	if math.Abs(bonuses.NavalHPBonusPct-0.10) > 0.001 {
		t.Errorf("Legendary 5* HP Bonus: got %.3f, expected 0.10", bonuses.NavalHPBonusPct)
	}
	if math.Abs(bonuses.NavalSpeedBonusPct-0.05) > 0.001 {
		t.Errorf("Legendary 5* Speed Bonus: got %.3f, expected 0.05", bonuses.NavalSpeedBonusPct)
	}
	if math.Abs(bonuses.RumConsumptionReductionPct-0.05) > 0.001 {
		t.Errorf("Legendary 5* Rum reduction: got %.3f, expected 0.05", bonuses.RumConsumptionReductionPct)
	}

	// 2. Common 1 Star
	// HP: 1.0% * 1 = 1%
	// Rum: 0.5% * 1 = 0.5%
	capCommon := domain.Captain{
		Rarity: domain.RarityCommon,
		Stars:  1,
	}
	bonusesCommon := ComputeNavalBonuses(capCommon)
	if math.Abs(bonusesCommon.NavalHPBonusPct-0.01) > 0.001 {
		t.Errorf("Common 1* HP Bonus: got %.3f, expected 0.01", bonusesCommon.NavalHPBonusPct)
	}
	if math.Abs(bonusesCommon.RumConsumptionReductionPct-0.005) > 0.001 {
		t.Errorf("Common 1* Rum reduction: got %.3f, expected 0.005", bonusesCommon.RumConsumptionReductionPct)
	}
}
