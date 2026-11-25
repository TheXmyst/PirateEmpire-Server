package auth

import (
	"net/http"
	"strings"

	"github.com/TheXmyst/Sea-Dogs/server/internal/domain"
	"github.com/TheXmyst/Sea-Dogs/server/internal/repository"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

// AuthContextKey is the key used to store authenticated player in Echo context
const AuthContextKey = "authenticated_player"

// TokenPlayerIDKey stocke l'id du joueur lu dans le jeton (utilisé par RequireToken).
const TokenPlayerIDKey = "token_player_id"

// loadPlayer charge un joueur depuis la base. C'est une variable pour que les
// tests puissent la remplacer sans base de données.
var loadPlayer = func(id uuid.UUID) (*domain.Player, error) {
	var p domain.Player
	if err := repository.GetDB().First(&p, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &p, nil
}

// bearerClaims lit l'en-tête "Authorization: Bearer <jeton>" et vérifie le jeton.
// Renvoie nil si l'en-tête est absent ou si le jeton est invalide/expiré.
func bearerClaims(c echo.Context) *TokenClaims {
	authHeader := c.Request().Header.Get("Authorization")
	if !strings.HasPrefix(authHeader, "Bearer ") {
		return nil
	}
	claims, err := ValidateToken(strings.TrimPrefix(authHeader, "Bearer "))
	if err != nil {
		return nil
	}
	return claims
}

func unauthorized(c echo.Context) error {
	return c.JSON(http.StatusUnauthorized, map[string]string{"error": "Authentication required"})
}

// RequireAuth exige un jeton valide ("Authorization: Bearer ...") et charge le
// joueur correspondant depuis la base.
//
// Il n'existe plus aucun autre moyen de s'authentifier : l'ancien mode
// "?player_id=..." (qui permettait de piloter n'importe quel compte en
// connaissant seulement son identifiant) a été supprimé.
func RequireAuth(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		claims := bearerClaims(c)
		if claims == nil {
			return unauthorized(c)
		}
		player, err := loadPlayer(claims.PlayerID)
		if err != nil || player == nil {
			return unauthorized(c)
		}
		c.Set(AuthContextKey, player)
		c.Set(TokenPlayerIDKey, player.ID)
		return next(c)
	}
}

// RequireToken vérifie seulement le jeton, sans lire la base. Il sert aux
// routes très fréquentes (comme /status, appelée toutes les 1-2 s) qui lisent
// l'état en mémoire et n'ont besoin que de l'id du joueur.
func RequireToken(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		claims := bearerClaims(c)
		if claims == nil {
			return unauthorized(c)
		}
		c.Set(TokenPlayerIDKey, claims.PlayerID)
		return next(c)
	}
}

// GetAuthenticatedPlayer extracts the authenticated player from Echo context
// Returns nil if no player is authenticated
func GetAuthenticatedPlayer(c echo.Context) *domain.Player {
	player, ok := c.Get(AuthContextKey).(*domain.Player)
	if !ok {
		return nil
	}
	return player
}

// GetTokenPlayerID renvoie l'id du joueur authentifié (posé par RequireAuth ou
// RequireToken), ou uuid.Nil si la requête n'est pas authentifiée.
func GetTokenPlayerID(c echo.Context) uuid.UUID {
	id, ok := c.Get(TokenPlayerIDKey).(uuid.UUID)
	if !ok {
		return uuid.Nil
	}
	return id
}
