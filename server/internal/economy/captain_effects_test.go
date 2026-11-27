package economy

import (
	"testing"

	"github.com/TheXmyst/Sea-Dogs/server/internal/domain"
	"github.com/google/uuid"
)

func ptr(v int) *int { return &v }

func TestEngagementMoraleFloorAfterEnemyDamage(t *testing.T) {
	a := domain.Fleet{ID: uuid.New(), IslandID: uuid.New(), State: domain.FleetStateMoving, MoraleCruise: ptr(50),
		Cargo: map[domain.ResourceType]float64{domain.Rum: 10}}
	b := domain.Fleet{ID: uuid.New(), MoraleCruise: ptr(40)}           // PNJ : pas de pénalité de rhum
	floorCapt := &domain.Captain{Level: 1, SkillID: "morale_floor"}    // plancher 15
	terror := &domain.Captain{Level: 80, SkillID: "terror_engagement"} // -30
	r := ComputeEngagementMorale(a, b, floorCapt, terror)
	if r.EngagementMoraleA != 20 { // 50 - 30 = 20 (au-dessus du plancher 15)
		t.Errorf("A morale = %d, want 20", r.EngagementMoraleA)
	}
	if r.EngagementMoraleB != 40 {
		t.Errorf("NPC morale = %d, want 40 (no rum penalty)", r.EngagementMoraleB)
	}
	ghost := &domain.Captain{Level: 1, SkillID: "ul_ghost_admiral"} // plancher 50
	r = ComputeEngagementMorale(a, b, ghost, terror)
	if r.EngagementMoraleA != 50 {
		t.Errorf("floor must apply after enemy damage: got %d", r.EngagementMoraleA)
	}
}

func TestEngagementRumPenaltyOnlyAtSea(t *testing.T) {
	atSea := domain.Fleet{ID: uuid.New(), IslandID: uuid.New(), State: domain.FleetStateMoving, MoraleCruise: ptr(50)}
	atPort := domain.Fleet{ID: uuid.New(), IslandID: uuid.New(), State: domain.FleetStateIdle, MoraleCruise: ptr(50)}
	r := ComputeEngagementMorale(atSea, atPort, nil, nil)
	if r.EngagementMoraleA != 30 || r.EngagementMoraleB != 50 {
		t.Errorf("got A=%d B=%d, want 30/50", r.EngagementMoraleA, r.EngagementMoraleB)
	}
	if r.AtkMultB <= 1 {
		t.Errorf("defender should get the morale bonus")
	}
}

func TestMoraleBonusModifiers(t *testing.T) {
	a := domain.Fleet{ID: uuid.New(), MoraleCruise: ptr(70)}
	b := domain.Fleet{ID: uuid.New(), MoraleCruise: ptr(50)}
	base := ComputeEngagementMorale(a, b, nil, nil).BonusPercent // écart 20 -> 18 %
	amp := ComputeEngagementMorale(a, b, &domain.Captain{Level: 80, SkillID: "enemy_morale_decay_multiplier"}, nil).BonusPercent
	red := ComputeEngagementMorale(a, b, nil, &domain.Captain{Level: 80, SkillID: "panic_immunity_threshold"}).BonusPercent
	if !(amp > base && red < base) {
		t.Errorf("base=%.3f amp=%.3f red=%.3f", base, amp, red)
	}
}

func TestMoraleRates(t *testing.T) {
	port := domain.Fleet{State: domain.FleetStateIdle, MoraleCruise: ptr(50)}
	if r := MoraleRatePerSecond(&port, CaptainEffects{}); r <= 0 {
		t.Errorf("port should recover, got %f", r)
	}
	sea := domain.Fleet{State: domain.FleetStateMoving, MoraleCruise: ptr(50), Cargo: map[domain.ResourceType]float64{domain.Rum: 5}}
	noRum := domain.Fleet{State: domain.FleetStateMoving, MoraleCruise: ptr(50)}
	if !(MoraleRatePerSecond(&noRum, CaptainEffects{}) < MoraleRatePerSecond(&sea, CaptainEffects{})) {
		t.Errorf("no rum should decay faster")
	}
	if MoraleRatePerSecond(&sea, CaptainEffects{NoSeaDecay: true}) != 0 {
		t.Errorf("storm king: no decay")
	}
}

func TestCaptainXP(t *testing.T) {
	c := &domain.Captain{Level: 1}
	if gained := AddCaptainXP(c, 40); gained != 1 || c.Level != 2 || c.XP != 0 {
		t.Errorf("lvl=%d xp=%d gained=%d", c.Level, c.XP, gained)
	}
	AddCaptainXP(c, 10_000_000)
	if c.Level != CaptainMaxLevel || c.XP != 0 {
		t.Errorf("max level: lvl=%d xp=%d", c.Level, c.XP)
	}
}

func TestFleetCaptainPrefersShipWithCaptain(t *testing.T) {
	capID := uuid.New()
	s1 := domain.Ship{ID: uuid.MustParse("00000000-0000-0000-0000-000000000001")}
	s2 := domain.Ship{ID: uuid.MustParse("00000000-0000-0000-0000-000000000002"), CaptainID: &capID}
	f := domain.Fleet{Ships: []domain.Ship{s2, s1}, FlagshipShipID: &s1.ID}
	c := FleetCaptain(&f, []domain.Captain{{ID: capID, Name: "X"}})
	if c == nil || c.ID != capID {
		t.Fatalf("expected the captain of the ship that has one")
	}
	if f.Ships[0].ID != s2.ID {
		t.Errorf("SelectFlagshipShip must not reorder ships")
	}
}

func TestWindWithCaptain(t *testing.T) {
	if ApplyWindWithCaptain(CaptainEffects{WindNeverUnfavorable: true}, 0.7) != 1 {
		t.Errorf("wind never unfavorable")
	}
	if v := ApplyWindWithCaptain(CaptainEffects{UnfavorableWindReduction: 0.5}, 0.8); v < 0.899 || v > 0.901 {
		t.Errorf("half penalty: %f", v)
	}
}
