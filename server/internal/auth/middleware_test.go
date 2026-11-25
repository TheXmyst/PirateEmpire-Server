package auth

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/TheXmyst/Sea-Dogs/server/internal/domain"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

// fakePlayers remplace la base de données pendant les tests.
func fakePlayers(t *testing.T, players ...domain.Player) {
	t.Helper()
	old := loadPlayer
	loadPlayer = func(id uuid.UUID) (*domain.Player, error) {
		for i := range players {
			if players[i].ID == id {
				return &players[i], nil
			}
		}
		return nil, errors.New("not found")
	}
	t.Cleanup(func() { loadPlayer = old })
}

// run exécute le middleware sur une requête et renvoie le code HTTP et le
// joueur vu par le handler.
func run(t *testing.T, mw echo.MiddlewareFunc, target, authHeader string) (int, uuid.UUID) {
	t.Helper()
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, target, nil)
	if authHeader != "" {
		req.Header.Set("Authorization", authHeader)
	}
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	var seen uuid.UUID
	h := mw(func(c echo.Context) error {
		seen = GetTokenPlayerID(c)
		return c.NoContent(http.StatusOK)
	})
	if err := h(c); err != nil {
		t.Fatalf("handler error: %v", err)
	}
	return rec.Code, seen
}

func TestRequireAuth_RejectsLegacyPlayerIDParam(t *testing.T) {
	admin := domain.Player{ID: uuid.New(), Username: "boss", IsAdmin: true}
	fakePlayers(t, admin)
	// Même avec l'ancienne variable d'environnement, le mode legacy n'existe plus.
	t.Setenv("ALLOW_LEGACY_PLAYER_ID_AUTH", "true")

	code, _ := run(t, RequireAuth, "/build?player_id="+admin.ID.String(), "")
	if code != http.StatusUnauthorized {
		t.Fatalf("expected 401 with only ?player_id=, got %d", code)
	}
}

func TestRequireAuth_ValidToken(t *testing.T) {
	p := domain.Player{ID: uuid.New(), Username: "alice"}
	fakePlayers(t, p)
	token, _ := GenerateToken(p.ID, p.Username, false)

	code, seen := run(t, RequireAuth, "/build", "Bearer "+token)
	if code != http.StatusOK || seen != p.ID {
		t.Fatalf("expected 200 for alice, got %d (player %s)", code, seen)
	}
}

func TestRequireAuth_TokenWinsOverPlayerIDParam(t *testing.T) {
	alice := domain.Player{ID: uuid.New(), Username: "alice"}
	bob := domain.Player{ID: uuid.New(), Username: "bob"}
	fakePlayers(t, alice, bob)
	token, _ := GenerateToken(alice.ID, alice.Username, false)

	_, seen := run(t, RequireAuth, "/build?player_id="+bob.ID.String(), "Bearer "+token)
	if seen != alice.ID {
		t.Fatalf("expected alice from token, got %s", seen)
	}
}

func TestRequireAuth_BadTokenOrDeletedPlayer(t *testing.T) {
	fakePlayers(t) // aucun joueur en base
	ghost, _ := GenerateToken(uuid.New(), "ghost", false)

	for name, header := range map[string]string{
		"garbage":        "Bearer not-a-token",
		"no bearer":      ghost,
		"deleted player": "Bearer " + ghost,
	} {
		if code, _ := run(t, RequireAuth, "/build", header); code != http.StatusUnauthorized {
			t.Errorf("%s: expected 401, got %d", name, code)
		}
	}
}

func TestRequireToken(t *testing.T) {
	id := uuid.New()
	token, _ := GenerateToken(id, "alice", false)

	if code, _ := run(t, RequireToken, "/status?player_id="+id.String(), ""); code != http.StatusUnauthorized {
		t.Fatalf("expected 401 without token, got %d", code)
	}
	code, seen := run(t, RequireToken, "/status", "Bearer "+token)
	if code != http.StatusOK || seen != id {
		t.Fatalf("expected 200 for token owner, got %d (player %s)", code, seen)
	}
}

func TestInitSecret(t *testing.T) {
	t.Cleanup(func() { tokenSecret = nil })

	t.Setenv("AUTH_SECRET", "")
	if err := InitSecret(false); err == nil {
		t.Error("expected error when AUTH_SECRET is missing in production")
	}
	if err := InitSecret(true); err != nil || len(tokenSecret) == 0 {
		t.Errorf("dev mode should generate a random secret, got err=%v", err)
	}

	t.Setenv("AUTH_SECRET", "trop-court")
	if err := InitSecret(true); err == nil {
		t.Error("expected error for a short AUTH_SECRET")
	}

	long := "une-cle-suffisamment-longue-pour-signer-les-jetons"
	t.Setenv("AUTH_SECRET", long)
	if err := InitSecret(false); err != nil || string(tokenSecret) != long {
		t.Errorf("expected AUTH_SECRET to be used, got err=%v", err)
	}
}
