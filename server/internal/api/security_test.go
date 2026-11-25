package api

import (
	"net/http/httptest"
	"testing"

	"github.com/TheXmyst/Sea-Dogs/server/internal/auth"
	"github.com/TheXmyst/Sea-Dogs/server/internal/domain"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestValidUsername(t *testing.T) {
	ok := []string{"TheXmyst", "Psy_Ko-42", "abc", "a2345678901234567890"}
	bad := []string{"", "ab", "a23456789012345678901", "avec espace", "pipe|name", "émile", "admin​"}
	for _, u := range ok {
		if !validUsername(u) {
			t.Errorf("%q should be accepted", u)
		}
	}
	for _, u := range bad {
		if validUsername(u) {
			t.Errorf("%q should be rejected", u)
		}
	}
}

func TestOwnsIsland(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file::memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&domain.Island{}); err != nil {
		t.Fatal(err)
	}
	alice := &domain.Player{ID: uuid.New(), Username: "alice"}
	bob := &domain.Player{ID: uuid.New(), Username: "bob"}
	aliceIsland := domain.Island{ID: uuid.New(), PlayerID: alice.ID, SeaID: uuid.New()}
	if err := db.Create(&aliceIsland).Error; err != nil {
		t.Fatal(err)
	}

	ctxFor := func(p *domain.Player) echo.Context {
		c := echo.New().NewContext(httptest.NewRequest("POST", "/", nil), httptest.NewRecorder())
		if p != nil {
			c.Set(auth.AuthContextKey, p)
		}
		return c
	}

	if !ownsIsland(ctxFor(alice), db, aliceIsland.ID) {
		t.Error("alice should own her island")
	}
	if ownsIsland(ctxFor(bob), db, aliceIsland.ID) {
		t.Error("bob must not be able to use alice's island")
	}
	if ownsIsland(ctxFor(nil), db, aliceIsland.ID) {
		t.Error("an unauthenticated request must not own anything")
	}
}
