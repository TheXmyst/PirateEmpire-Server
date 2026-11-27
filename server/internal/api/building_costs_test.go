package api

import (
	"testing"

	"github.com/TheXmyst/Sea-Dogs/server/internal/api/dto"
	"github.com/TheXmyst/Sea-Dogs/server/internal/economy"
)

// Les coûts envoyés au client doivent être exactement ceux que le serveur
// débite (bug : le client affichait HdV 3 -> 4 à 2767 or au lieu de 1647).
func TestFillBuildingCosts(t *testing.T) {
	if err := economy.LoadConfig("../../configs/buildings.json"); err != nil {
		t.Fatalf("config: %v", err)
	}
	island := &dto.IslandDTO{Buildings: []dto.BuildingDTO{{Type: "Hôtel de Ville", Level: 3}}}
	fillBuildingCosts(island)

	stats, _ := economy.GetBuildingStats("Hôtel de Ville", 4)
	got := island.Buildings[0].NextLevelCost["gold"]
	if got != stats.Cost["gold"] || got != 1647 {
		t.Fatalf("coût HdV 4 : got %v, want %v (1647)", got, stats.Cost["gold"])
	}
	if c := island.BuildCosts["Entrepôt"]; c["gold"] != 400 {
		t.Fatalf("coût Entrepôt niv. 1 : got %v, want 400 or", c)
	}
	if c := island.BuildCosts["Scierie"]; c["gold"] != 0 {
		t.Fatalf("la Scierie ne coûte pas d'or : got %v", c)
	}
}
