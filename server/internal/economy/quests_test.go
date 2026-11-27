package economy

import (
	"testing"

	"github.com/TheXmyst/Sea-Dogs/server/internal/domain"
	"github.com/google/uuid"
)

func loadQuestTestConfig(t *testing.T) {
	t.Helper()
	if err := LoadConfig("../../configs/buildings.json"); err != nil {
		t.Fatalf("buildings.json: %v", err)
	}
	if err := LoadQuestConfig("../../configs/quests.json"); err != nil {
		t.Fatalf("quests.json: %v", err)
	}
}

func TestQuestConfigShape(t *testing.T) {
	loadQuestTestConfig(t)
	count := map[string]int{}
	tickets := 0
	for _, d := range QuestDefs() {
		count[d.Family]++
		tickets += d.Reward.Tickets
	}
	if count[QuestFamilyStory] != 15 || count[QuestFamilyBuilding] != 96 || count[QuestFamilyChallenge] != 24 {
		t.Errorf("familles : %v (attendu 15 / 96 / 24)", count)
	}
	if tickets != 13 {
		t.Errorf("tickets : %d (attendu 13)", tickets)
	}
	// Remboursement Bâtiments : 50 % jusqu'au niveau 5, 6 % ensuite.
	d, ok := GetQuestDef("bld:Hôtel de Ville:2")
	if !ok || d.Reward.Gold != 470 || d.Phase != 1 {
		t.Errorf("HdV 2 : %+v", d)
	}
	d, _ = GetQuestDef("bld:Hôtel de Ville:7")
	if d.Phase != 2 || d.Reward.Gold != 230 {
		t.Errorf("HdV 7 : %+v", d.Reward)
	}
	if d, _ := GetQuestDef("bld:Scierie:1"); d.Reward.Wood != 50 || d.Title != "Construire : Scierie" {
		t.Errorf("Scierie 1 : %+v", d)
	}
	if d, _ := GetQuestDef("bld:Tavern:2"); d.Title != "Taverne niveau 2" {
		t.Errorf("nom affiché de la Taverne : %q", d.Title)
	}
}

// Le calibrage annoncé : ~90 % du coût jusqu'au niveau 5, ~20 % ensuite.
func TestQuestRewardBudget(t *testing.T) {
	loadQuestTestConfig(t)
	var phase [3]QuestReward
	for _, d := range QuestDefs() {
		phase[d.Phase] = phase[d.Phase].Add(d.Reward)
	}
	check := func(name string, got, lo, hi float64) {
		if got < lo || got > hi {
			t.Errorf("%s = %.0f, attendu entre %.0f et %.0f", name, got, lo, hi)
		}
	}
	// Dépenses jusqu'au niveau 5 ≈ 33k bois, 20k pierre, 46k or.
	check("phase 1 bois", phase[1].Wood, 0.80*33000, 1.0*33000)
	check("phase 1 pierre", phase[1].Stone, 0.80*20000, 1.0*20000)
	check("phase 1 or", phase[1].Gold, 0.85*46000, 1.05*46000)
	// Niveaux 6 à 8 ≈ 52,6k bois, 39,7k pierre, 74,7k or : petit coup de pouce.
	check("phase 2 bois", phase[2].Wood, 0.10*52600, 0.30*52600)
	check("phase 2 pierre", phase[2].Stone, 0.10*39700, 0.30*39700)
	check("phase 2 or", phase[2].Gold, 0.10*74700, 0.30*74700)
}

func newQuestState() *QuestPlayerState {
	return &QuestPlayerState{
		PveWins:   map[int]int{},
		Counters:  map[string]float64{},
		Claimed:   map[string]bool{},
	}
}

func visibleIDs(s *QuestPlayerState) map[string]QuestStatus {
	qs, _ := EvaluateQuests(s)
	out := map[string]QuestStatus{}
	for _, q := range qs {
		out[q.ID] = q
	}
	return out
}

func TestQuestVisibilityNewPlayer(t *testing.T) {
	loadQuestTestConfig(t)
	s := newQuestState()
	v := visibleIDs(s)

	// Histoire : seulement la première quête.
	if _, ok := v["story_first_building"]; !ok {
		t.Error("la première quête Histoire doit être visible")
	}
	if _, ok := v["story_first_research"]; ok {
		t.Error("la deuxième quête Histoire ne doit pas encore être visible")
	}
	// Bâtiments : le niveau suivant de chaque bâtiment, verrouillé si besoin.
	if q, ok := v["bld:Hôtel de Ville:1"]; !ok || q.Locked {
		t.Errorf("Hôtel de Ville 1 doit être visible et disponible : %+v", q)
	}
	if q, ok := v["bld:Scierie:1"]; !ok || !q.Locked {
		t.Errorf("Scierie 1 doit attendre l'Hôtel de Ville : %+v", q)
	}
	if q, ok := v["bld:Académie:1"]; !ok || !q.Locked || q.Hint == "" {
		t.Errorf("Académie 1 doit être verrouillée avec un indice : %+v", q)
	}
	if _, ok := v["bld:Scierie:2"]; ok {
		t.Error("Scierie 2 ne doit pas être visible avant la Scierie 1")
	}
	// Défis : le chat est ouvert à tous, les recherches attendent l'HdV 3.
	if _, ok := v["ch_chat"]; !ok {
		t.Error("le défi du chat doit être visible")
	}
	if _, ok := v["ch_tech_3"]; ok {
		t.Error("le défi des recherches attend l'Hôtel de Ville 3")
	}
	if n := len(ClaimableQuests(s)); n != 0 {
		t.Errorf("rien à réclamer pour un nouveau joueur, obtenu %d", n)
	}
}

func TestQuestProgressAndStoryChain(t *testing.T) {
	loadQuestTestConfig(t)
	s := newQuestState()
	s.Buildings = []domain.Building{{Type: "Hôtel de Ville", Level: 1}, {Type: "Scierie", Level: 3}}

	claimable := map[string]bool{}
	for _, d := range ClaimableQuests(s) {
		claimable[d.ID] = true
	}
	for _, id := range []string{"story_first_building", "bld:Scierie:1", "bld:Scierie:2", "bld:Scierie:3"} {
		if !claimable[id] {
			t.Errorf("%s devrait être réclamable", id)
		}
	}
	if claimable["bld:Scierie:4"] {
		t.Error("Scierie 4 n'est pas atteinte")
	}

	// Une construction en cours ne compte pas : Level = dernier niveau terminé.
	s.Buildings = append(s.Buildings, domain.Building{Type: "Mine d'Or", Level: 0, Constructing: true})
	if d, _ := GetQuestDef("bld:Mine d'Or:1"); QuestDone(d, s) {
		t.Error("une Mine d'Or en construction ne doit pas valider la quête")
	}

	// Réclamer la première quête Histoire ouvre la suivante.
	s.Claimed["story_first_building"] = true
	v := visibleIDs(s)
	if _, ok := v["story_first_research"]; !ok {
		t.Error("la deuxième quête Histoire doit s'ouvrir")
	}
	if _, ok := v["story_first_building"]; ok {
		t.Error("une quête réclamée ne doit plus être proposée")
	}
}

func TestQuestObjectives(t *testing.T) {
	loadQuestTestConfig(t)
	s := newQuestState()
	shipID := uuid.New()
	s.Ships = []domain.Ship{
		{ID: shipID, Type: "sloop", State: "Ready", MilitiaWarriors: 5},
		{ID: uuid.New(), Type: "sloop", State: "UnderConstruction"},
	}
	s.Captains = []domain.Captain{{Rarity: domain.RarityRare, AssignedShipID: &shipID}, {Rarity: domain.RarityCommon}}
	s.PveWins = map[int]int{1: 2, 2: 1}
	s.Counters[QuestCounterCaptainPulls] = 3

	cases := []struct {
		obj  QuestObjective
		want float64
	}{
		{QuestObjective{Type: "ships_owned", Count: 2}, 1}, // le navire en construction ne compte pas
		{QuestObjective{Type: "ship_crewed", Count: 1}, 1},
		{QuestObjective{Type: "captain_rarity", Rarity: "rare", Count: 1}, 1},
		{QuestObjective{Type: "captains_owned", Count: 3}, 2},
		{QuestObjective{Type: "pve_wins", Tier: 1, Count: 3}, 2},
		{QuestObjective{Type: "pve_wins", Count: 10}, 3},
		{QuestObjective{Type: "counter", Counter: QuestCounterCaptainPulls, Count: 5}, 3},
		{QuestObjective{Type: "militia_all_types"}, 1},
	}
	for _, c := range cases {
		got, _ := QuestProgress(QuestDef{Objective: c.obj}, s)
		if got != c.want {
			t.Errorf("%+v : progression %.0f, attendu %.0f", c.obj, got, c.want)
		}
	}
}

func TestComputeStorageLimits(t *testing.T) {
	loadQuestTestConfig(t)
	base := ComputeStorageLimits(nil, nil)
	if base[domain.Wood] != 5000 || base[domain.Gold] != 10000 {
		t.Errorf("capacité de base : %v", base)
	}
	withWarehouse := ComputeStorageLimits([]domain.Building{{Type: "Entrepôt", Level: 1}}, nil)
	if withWarehouse[domain.Wood] != 20000 || withWarehouse[domain.Gold] != 50000 {
		t.Errorf("Entrepôt 1 : %v", withWarehouse)
	}
}
