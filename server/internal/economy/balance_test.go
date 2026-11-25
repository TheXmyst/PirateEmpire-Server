package economy

import (
	"math"
	"testing"
	"time"

	"github.com/TheXmyst/Sea-Dogs/server/internal/domain"
)

func TestIsBeginnerProtected(t *testing.T) {
	now := time.Now()
	fresh := &domain.Player{CreatedAt: now.Add(-1 * time.Hour)}
	old := &domain.Player{CreatedAt: now.Add(-73 * time.Hour)}
	forfeited := &domain.Player{CreatedAt: now.Add(-1 * time.Hour), BeginnerProtectionForfeited: true}

	cases := []struct {
		name string
		p    *domain.Player
		th   int
		want bool
	}{
		{"nouveau joueur, HdV 3", fresh, 3, true},
		{"nouveau joueur, HdV 7", fresh, 7, true},
		{"nouveau joueur, HdV 8", fresh, 8, false},
		{"compte de plus de 72 h", old, 2, false},
		{"protection perdue en attaquant", forfeited, 2, false},
		{"joueur nil", nil, 1, false},
	}
	for _, c := range cases {
		if got := IsBeginnerProtected(c.p, c.th, now); got != c.want {
			t.Errorf("%s: got %v, want %v", c.name, got, c.want)
		}
	}
}

func TestComputePvpLoot(t *testing.T) {
	res := map[domain.ResourceType]float64{
		domain.Wood:          12000,
		domain.Gold:          3000, // sous la réserve (HdV 2 -> 4 000)
		domain.Rum:           8000,
		domain.CaptainTicket: 50, // jamais pillé
	}
	// Cale assez grande : 25 % de (ressource - 4 000)
	loot := ComputePvpLoot(res, 2, 100000)
	if loot[domain.Wood] != 2000 || loot[domain.Rum] != 1000 {
		t.Errorf("butin inattendu: %v", loot)
	}
	if _, ok := loot[domain.Gold]; ok {
		t.Errorf("l'or sous la réserve ne doit pas être pillé: %v", loot)
	}
	if _, ok := loot[domain.CaptainTicket]; ok {
		t.Errorf("les tickets ne doivent jamais être pillés")
	}

	// Cale limitée à 1 500 : le butin est réduit au prorata
	capped := ComputePvpLoot(res, 2, 1500)
	total := capped[domain.Wood] + capped[domain.Rum]
	if total > 1500 || total < 1498 {
		t.Errorf("butin plafonné attendu ~1500, obtenu %.0f (%v)", total, capped)
	}
	if math.Abs(capped[domain.Wood]-2*capped[domain.Rum]) > 2 {
		t.Errorf("la répartition doit rester proportionnelle: %v", capped)
	}

	// Cale pleine : rien
	if len(ComputePvpLoot(res, 2, 0)) != 0 {
		t.Errorf("cale pleine: aucun butin attendu")
	}
}

func TestComputeTicketRewards(t *testing.T) {
	// Nouveau compte, premier jour, HdV 1 : ticket quotidien seulement
	r := ComputeTicketRewards("", "2026-11-25", 0, 1)
	if r.Daily != 1 || r.TownHall != 0 || r.NewTownHallL != 1 || r.NewDailyDay != "2026-11-25" {
		t.Errorf("cas 1: %+v", r)
	}
	// Même jour, HdV passé de 1 à 4 : 3 niveaux x 2 tickets
	r = ComputeTicketRewards("2026-11-25", "2026-11-25", 1, 4)
	if r.Daily != 0 || r.TownHall != 6 || r.NewTownHallL != 4 {
		t.Errorf("cas 2: %+v", r)
	}
	// Compte existant jamais récompensé, HdV 10 : rattrapage des niveaux 2 à 10
	r = ComputeTicketRewards("", "2026-11-26", 0, 10)
	if r.Daily != 1 || r.TownHall != 18 || r.Total() != 19 {
		t.Errorf("cas 3: %+v", r)
	}
	// Rien de nouveau
	r = ComputeTicketRewards("2026-11-26", "2026-11-26", 10, 10)
	if r.Total() != 0 {
		t.Errorf("cas 4: %+v", r)
	}
}

func TestRumConsumptionRebalance(t *testing.T) {
	perHour := ComputeRumConsumption(1, false) * 3600
	if math.Abs(perHour-72) > 0.01 {
		t.Errorf("conso par navire attendue 72/h, obtenu %.2f", perHour)
	}
}

func TestShippedConfigsRebalance(t *testing.T) {
	if err := LoadConfig("../../configs/buildings.json"); err != nil {
		t.Fatalf("buildings.json: %v", err)
	}
	if err := LoadShipConfig("../../configs/ships.json"); err != nil {
		t.Fatalf("ships.json: %v", err)
	}

	// Distillerie : 112/h au niveau 10
	d, _ := GetBuildingStats("Distillerie", 10)
	if d.Production != 112 {
		t.Errorf("Distillerie niv. 10: attendu 112/h, obtenu %.0f", d.Production)
	}
	// Hôtel de Ville niveau 10 : 9 115 or
	th, _ := GetBuildingStats("Hôtel de Ville", 10)
	if math.Abs(th.Cost[domain.Gold]-9115) > 1 {
		t.Errorf("HdV niv. 10: attendu 9115 or, obtenu %.0f", th.Cost[domain.Gold])
	}
	// La courbe de construction se raccorde au niveau 10 (5 h pour l'HdV)
	if th.BuildTime < 4*time.Hour+59*time.Minute || th.BuildTime > 5*time.Hour+time.Minute {
		t.Errorf("HdV niv. 10: attendu ~5 h, obtenu %s", th.BuildTime)
	}
	th1, _ := GetBuildingStats("Hôtel de Ville", 1)
	if th1.BuildTime != time.Minute {
		t.Errorf("HdV niv. 1: attendu 1 min, obtenu %s", th1.BuildTime)
	}
	// Le Sloop est désormais le navire le plus rapide
	sloop, _ := GetShipStats("sloop")
	frigate, _ := GetShipStats("frigate")
	if sloop.BaseSpeed <= frigate.BaseSpeed {
		t.Errorf("le Sloop (%.1f) doit être plus rapide que la Frégate (%.1f)", sloop.BaseSpeed, frigate.BaseSpeed)
	}
	if ShipCargoCapacity("frigate") != frigate.CargoCapacity {
		t.Errorf("la cale doit venir de ships.json")
	}
}
