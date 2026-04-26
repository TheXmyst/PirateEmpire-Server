package economy

import (
	"testing"

	"github.com/TheXmyst/Sea-Dogs/server/internal/domain"
)

func setupMockShips() {
	ShipInfos = make(map[string]ShipConfig)
	ShipInfos["sloop"] = ShipConfig{
		Name:      "Sloop",
		MaxHealth: 100,
		Cost: map[domain.ResourceType]float64{
			domain.Wood: 12000,
			domain.Gold: 4000,
		},
	}
	ShipInfos["frigate"] = ShipConfig{
		Name:      "Frigate",
		MaxHealth: 500,
		Cost: map[domain.ResourceType]float64{
			domain.Wood: 50000,
			domain.Gold: 25000,
			domain.Rum:  5000,
		},
	}
}

func TestCalculateScrapReward(t *testing.T) {
	setupMockShips()

	// Test Sloop (Low value)
	rewards, err := CalculateScrapReward("sloop")
	if err != nil {
		t.Fatalf("Failed to calculate reward for sloop: %v", err)
	}

	// Sloop cost: 12000 Wood, 4000 Gold
	// Loot should be ~2400 Wood, ~800 Gold
	// Allow variance of +/- 15% from base 20%

	expectedWood := 12000.0 * 0.20
	if rewards[domain.Wood] < expectedWood*0.8 || rewards[domain.Wood] > expectedWood*1.2 {
		t.Errorf("Sloop Wood reward out of range: got %.0f, expected ~%.0f", rewards[domain.Wood], expectedWood)
	}

	expectedGold := 4000.0 * 0.20
	if rewards[domain.Gold] < expectedGold*0.8 || rewards[domain.Gold] > expectedGold*1.2 {
		t.Errorf("Sloop Gold reward out of range: got %.0f, expected ~%.0f", rewards[domain.Gold], expectedGold)
	}

	// Test Frigate (Mid value)
	rewards, err = CalculateScrapReward("frigate")
	if err != nil {
		t.Fatalf("Failed to calculate reward for frigate: %v", err)
	}

	// Frigate cost: 50000 Wood, 25000 Gold, 5000 Rum
	expectedRum := 5000.0 * 0.20
	if rewards[domain.Rum] < expectedRum*0.8 || rewards[domain.Rum] > expectedRum*1.2 {
		t.Errorf("Frigate Rum reward out of range: got %.0f, expected ~%.0f", rewards[domain.Rum], expectedRum)
	}
}

func TestCalculateCombatRewards(t *testing.T) {
	setupMockShips()

	destroyed := []string{"sloop", "sloop"}
	total := CalculateCombatRewards(destroyed)

	// Should be roughly double the sloop reward
	expectedWood := 2 * 12000.0 * 0.20
	if total[domain.Wood] < expectedWood*0.8 || total[domain.Wood] > expectedWood*1.2 {
		t.Errorf("Total Wood reward out of range: got %.0f, expected ~%.0f", total[domain.Wood], expectedWood)
	}
}

func TestCalculateScrapReward_UnknownShip(t *testing.T) {
	setupMockShips()

	_, err := CalculateScrapReward("ufo_mothership")
	if err == nil {
		t.Error("Expected error for unknown ship type, got nil")
	}
}
