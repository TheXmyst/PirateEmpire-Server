package economy

import (
	"math"
	"testing"

	"github.com/TheXmyst/Sea-Dogs/server/internal/domain"
)

// Mock tech loading for testing
func setupMockTechs() {
	// Initialize map manually for test since we can't load file
	techMap = make(map[string]Technology)
	techMap["mock_wood_1"] = Technology{
		ID:      "mock_wood_1",
		Effects: TechEffect{ProdWood: 0.05},
	}
	techMap["mock_wood_2"] = Technology{
		ID:      "mock_wood_2",
		Effects: TechEffect{ProdWood: 0.10},
	}
	techMap["mock_naval_1"] = Technology{
		ID:      "mock_naval_1",
		Effects: TechEffect{SpeedBonus: 0.05, WindBonus: 0.1},
	}
	techMap["mock_combat_1"] = Technology{
		ID:      "mock_combat_1",
		Effects: TechEffect{CrewHP: 0.1, GuerrierBonus: 0.2},
	}
}

func assertEqualCounts(t *testing.T, expected, actual float64, name string) {
	if math.Abs(expected-actual) > 0.0001 {
		t.Errorf("%s: expected %f, got %f", name, expected, actual)
	}
}

func TestComputeTechModifiers_Economy(t *testing.T) {
	setupMockTechs()

	// Case 1: Single Tech
	mods := ComputeTechModifiers([]string{"mock_wood_1"})

	if mods.ResourceProductionMultiplier[domain.Wood] != 0.05 {
		t.Errorf("Expected 0.05 Wood bonus, got %f", mods.ResourceProductionMultiplier[domain.Wood])
	}
	if mods.ResourceProductionMultiplier[domain.Gold] != 0.0 {
		t.Errorf("Expected 0.0 Gold bonus, got %f", mods.ResourceProductionMultiplier[domain.Gold])
	}

	// Case 2: Stacking (Additive)
	mods = ComputeTechModifiers([]string{"mock_wood_1", "mock_wood_2"})
	assertEqualCounts(t, 0.15, mods.ResourceProductionMultiplier[domain.Wood], " dWood Stacking")
}

func TestComputeTechModifiers_Naval(t *testing.T) {
	setupMockTechs()

	mods := ComputeTechModifiers([]string{"mock_naval_1"})
	assertEqualCounts(t, 0.05, mods.ShipSpeedMultiplier, "SpeedBonus")
	assertEqualCounts(t, 0.1, mods.WindEfficiency, "WindBonus")
}

func TestComputeTechModifiers_Combat(t *testing.T) {
	setupMockTechs()

	mods := ComputeTechModifiers([]string{"mock_combat_1"})
	assertEqualCounts(t, 0.1, mods.CrewStatsMultiplier["hp"], "CrewHP")
	assertEqualCounts(t, 0.2, mods.UnitTypeBonus[domain.Warrior], "WarriorBonus")
	assertEqualCounts(t, 0.0, mods.UnitTypeBonus[domain.Archer], "ArcherBonus")
}

func TestComputeTechModifiers_Empty(t *testing.T) {
	setupMockTechs()
	mods := ComputeTechModifiers([]string{})
	if len(mods.ResourceProductionMultiplier) != 0 {
		t.Error("Expected empty map")
	}
}

func TestComputeTechModifiers_UnknownID(t *testing.T) {
	setupMockTechs()
	mods := ComputeTechModifiers([]string{"unknown_tech_999"})
	if len(mods.ResourceProductionMultiplier) != 0 {
		t.Error("Expected empty map")
	}
}
