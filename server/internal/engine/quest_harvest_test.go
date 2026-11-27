package engine

import (
	"testing"
	"time"

	"github.com/TheXmyst/Sea-Dogs/server/internal/domain"
	"github.com/TheXmyst/Sea-Dogs/server/internal/economy"
)

// La récolte sur un spot alimente le compteur de la quête « Récolter ».
func TestHarvestFeedsQuestCounter(t *testing.T) {
	if err := economy.LoadShipConfig("../../configs/ships.json"); err != nil {
		t.Fatal(err)
	}
	since := time.Now().Add(-time.Minute)
	island := &domain.Island{Fleets: []domain.Fleet{{
		Name:           "Test",
		State:          domain.FleetStateStationed,
		StationedAt:    &since,
		StoredResource: string(domain.Wood),
		NodeRichness:   1,
		Ships:          []domain.Ship{{Type: "sloop"}},
	}}}
	UpdateFleetStationing(island, 10)
	got := island.Player.QuestHarvested
	cargo := island.Fleets[0].Cargo[domain.Wood]
	if got <= 0 || got != cargo {
		t.Errorf("récolté pour les quêtes = %.2f, cale = %.2f", got, cargo)
	}
}
