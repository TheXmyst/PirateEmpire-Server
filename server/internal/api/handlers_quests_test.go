package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/TheXmyst/Sea-Dogs/server/internal/auth"
	"github.com/TheXmyst/Sea-Dogs/server/internal/domain"
	"github.com/TheXmyst/Sea-Dogs/server/internal/economy"
	"github.com/TheXmyst/Sea-Dogs/server/internal/repository"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupQuestDB(t *testing.T) (*domain.Player, *domain.Island) {
	t.Helper()
	if err := economy.LoadConfig("../../configs/buildings.json"); err != nil {
		t.Fatal(err)
	}
	if err := economy.LoadQuestConfig("../../configs/quests.json"); err != nil {
		t.Fatal(err)
	}
	db, err := gorm.Open(sqlite.Open("file:"+uuid.NewString()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&domain.Player{}, &domain.Island{}, &domain.Building{}, &domain.Ship{}, &domain.Fleet{},
		&domain.Captain{}, &domain.PvEVictory{}, &domain.PlayerQuestClaim{}); err != nil {
		t.Fatal(err)
	}
	old := repository.DB
	repository.DB = db
	t.Cleanup(func() { repository.DB = old })

	p := &domain.Player{ID: uuid.New(), Username: "kevin"}
	if err := db.Create(p).Error; err != nil {
		t.Fatal(err)
	}
	island := &domain.Island{ID: uuid.New(), PlayerID: p.ID, SeaID: uuid.New(),
		Resources: map[domain.ResourceType]float64{domain.Wood: 1000, domain.Stone: 1000, domain.Gold: 1000}}
	if err := db.Create(island).Error; err != nil {
		t.Fatal(err)
	}
	for _, b := range []domain.Building{
		{ID: uuid.New(), IslandID: island.ID, Type: "Hôtel de Ville", Level: 1},
		{ID: uuid.New(), IslandID: island.ID, Type: "Scierie", Level: 2},
	} {
		if err := db.Create(&b).Error; err != nil {
			t.Fatal(err)
		}
	}
	return p, island
}

func questCall(t *testing.T, h echo.HandlerFunc, p *domain.Player, body string) (int, map[string]interface{}) {
	t.Helper()
	method := http.MethodGet
	if body != "" {
		method = http.MethodPost
	}
	req := httptest.NewRequest(method, "/", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := echo.New().NewContext(req, rec)
	c.Set(auth.AuthContextKey, p)
	if err := h(c); err != nil {
		t.Fatal(err)
	}
	var out map[string]interface{}
	_ = json.Unmarshal(rec.Body.Bytes(), &out)
	return rec.Code, out
}

func islandWood(t *testing.T, id uuid.UUID) float64 {
	var isl domain.Island
	if err := repository.DB.First(&isl, "id = ?", id).Error; err != nil {
		t.Fatal(err)
	}
	return isl.Resources[domain.Wood]
}

func TestClaimQuestFlow(t *testing.T) {
	p, island := setupQuestDB(t)

	code, out := questCall(t, GetQuests, p, "")
	if code != 200 {
		t.Fatalf("GET /quests : %d %v", code, out)
	}
	// Histoire 1 + Scierie 1 et 2.
	if n := out["claimable"].(float64); n != 3 {
		t.Errorf("claimable = %v, attendu 3", n)
	}

	// Une quête pas terminée est refusée.
	if code, _ := questCall(t, ClaimQuest, p, `{"id":"bld:Scierie:3"}`); code != 400 {
		t.Errorf("Scierie 3 ne doit pas être réclamable (code %d)", code)
	}

	// Réclamer Scierie 1 : +50 bois.
	code, out = questCall(t, ClaimQuest, p, `{"id":"bld:Scierie:1"}`)
	if code != 200 {
		t.Fatalf("claim : %d %v", code, out)
	}
	if w := islandWood(t, island.ID); w != 1050 {
		t.Errorf("bois après Scierie 1 : %.0f, attendu 1050", w)
	}
	// Deux fois la même : refusé, rien de plus.
	if code, _ := questCall(t, ClaimQuest, p, `{"id":"bld:Scierie:1"}`); code != 400 {
		t.Errorf("double réclamation acceptée (code %d)", code)
	}
	if w := islandWood(t, island.ID); w != 1050 {
		t.Errorf("le bois a changé après une double réclamation : %.0f", w)
	}

	// Tout réclamer : Histoire 1 (300 bois) + Scierie 2 (70 bois).
	code, out = questCall(t, ClaimQuest, p, `{"all":true}`)
	if code != 200 || len(out["claimed"].([]interface{})) != 2 {
		t.Fatalf("claim all : %d %v", code, out)
	}
	if w := islandWood(t, island.ID); w != 1420 {
		t.Errorf("bois après tout réclamer : %.0f, attendu 1420", w)
	}
	if n := out["claimable"].(float64); n != 0 {
		t.Errorf("il ne doit plus rien rester à réclamer, obtenu %v", n)
	}
}

func TestClaimQuestRefusesWhenStorageFull(t *testing.T) {
	p, island := setupQuestDB(t)
	// Sans Entrepôt, le bois est limité à 5 000.
	repository.DB.Model(&domain.Island{}).Where("id = ?", island.ID).
		Update("resources", []byte(`{"wood":4990,"stone":0,"gold":0}`))
	code, out := questCall(t, ClaimQuest, p, `{"id":"bld:Scierie:1"}`)
	if code != 400 || !strings.Contains(out["error"].(string), "entrepôt") {
		t.Errorf("attendu un refus « entrepôt plein », obtenu %d %v", code, out)
	}
	if w := islandWood(t, island.ID); w != 4990 {
		t.Errorf("le bois ne doit pas bouger : %.0f", w)
	}
}

func TestQuestCounters(t *testing.T) {
	p, _ := setupQuestDB(t)
	ok := func(c echo.Context) error { return c.JSON(200, map[string]string{}) }
	fail := func(c echo.Context) error { return c.JSON(400, map[string]string{}) }
	mw := CountsForQuests(economy.QuestCounterChatMessages)
	questCall(t, mw(ok), p, `{}`)
	questCall(t, mw(fail), p, `{}`) // un échec ne compte pas
	BumpQuestCounter(repository.DB, p.ID, economy.QuestCounterMilitiaRecruited, 20)

	var got domain.Player
	repository.DB.First(&got, "id = ?", p.ID)
	if got.QuestChatMessages != 1 || got.QuestMilitiaRecruited != 20 {
		t.Errorf("compteurs : chat=%d matelots=%d", got.QuestChatMessages, got.QuestMilitiaRecruited)
	}
	_, out := questCall(t, GetQuests, p, "")
	found := false
	for _, q := range out["quests"].([]interface{}) {
		m := q.(map[string]interface{})
		if m["id"] == "ch_chat" && m["done"] == true {
			found = true
		}
	}
	if !found {
		t.Error("le défi du chat doit être terminé")
	}
}
