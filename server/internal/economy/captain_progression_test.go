package economy

import (
	"math"
	"testing"

	"github.com/TheXmyst/Sea-Dogs/server/internal/domain"
)

// TestPityLogic verifies that the Pity system correctly guarantees rarity
func TestPityLogic(t *testing.T) {
	if r, forced := RollCaptainRarityWithPity(PityUltraThreshold-1, 0, 0); r != domain.RarityUltra || !forced {
		t.Errorf("Expected forced UL at UL pity, got %s forced=%v", r, forced)
	}
	if r, forced := RollCaptainRarityWithPity(0, PityLegendaryThreshold-1, 0); r != domain.RarityLegendary || !forced {
		t.Errorf("Expected forced Legendary at legendary pity, got %s forced=%v", r, forced)
	}
	for i := 0; i < 200; i++ {
		if r, _ := RollCaptainRarityWithPity(0, 0, PityRareThreshold-1); r == domain.RarityCommon {
			t.Fatalf("Expected at least Rare at rare pity, got %s", r)
		}
	}
	if r, _ := RollCaptainRarityWithPity(0, 0, 0); r == "" {
		t.Errorf("Rolled empty rarity")
	}
}

func TestPityCountersReset(t *testing.T) {
	u, l, r := 5, 5, 5
	UpdatePityCounters(domain.RarityRare, &u, &l, &r)
	if u != 6 || l != 6 || r != 0 {
		t.Errorf("after rare: %d %d %d", u, l, r)
	}
	UpdatePityCounters(domain.RarityLegendary, &u, &l, &r)
	if u != 7 || l != 0 || r != 0 {
		t.Errorf("after legendary: %d %d %d", u, l, r)
	}
	UpdatePityCounters(domain.RarityUltra, &u, &l, &r)
	if u != 0 || l != 0 || r != 0 {
		t.Errorf("after UL: %d %d %d", u, l, r)
	}
}

func TestGachaRatesAndPool(t *testing.T) {
	sum := GachaRateCommon + GachaRateRare + GachaRateLegendary + GachaRateUltra
	if math.Abs(sum-1) > 1e-9 {
		t.Fatalf("rates sum to %f", sum)
	}
	want := map[domain.CaptainRarity]int{domain.RarityCommon: 16, domain.RarityRare: 12, domain.RarityLegendary: 8, domain.RarityUltra: 4}
	seen := map[string]bool{}
	total := 0
	for r, n := range want {
		ts := GetTemplatesByRarity(r)
		if len(ts) != n {
			t.Errorf("%s: %d templates, want %d", r, len(ts), n)
		}
		for _, tpl := range ts {
			if seen[tpl.TemplateID] {
				t.Errorf("duplicate template %s", tpl.TemplateID)
			}
			seen[tpl.TemplateID] = true
			total++
			c := domain.Captain{Level: 1, SkillID: tpl.SkillID, Rarity: tpl.Rarity}
			if lines := CaptainEffectLines(&c); len(lines) == 0 || lines[0] == "Aucun effet" {
				t.Errorf("template %s (%s) has no effect", tpl.TemplateID, tpl.SkillID)
			}
		}
	}
	if total < 40 {
		t.Errorf("only %d captains", total)
	}
	if rarityFromRoll(0.0) != domain.RarityCommon || rarityFromRoll(0.99) != domain.RarityUltra || rarityFromRoll(0.90) != domain.RarityLegendary {
		t.Errorf("rarityFromRoll boundaries wrong")
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
