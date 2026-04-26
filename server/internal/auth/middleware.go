package auth

import (
	"os"
	"strconv"
	"strings"

	"github.com/TheXmyst/Sea-Dogs/server/internal/domain"
	"github.com/TheXmyst/Sea-Dogs/server/internal/repository"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

// AuthContextKey is the key used to store authenticated player in Echo context
const AuthContextKey = "authenticated_player"

// RequireAuth is a middleware that validates authentication
// It checks for a Bearer token in the Authorization header first
// If no token is found and ALLOW_LEGACY_PLAYER_ID_AUTH is true, it falls back to checking player_id in query params (LEGACY - for development only)
// This middleware is STRICT: it returns 401 if no authenticated player can be resolved
func RequireAuth(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		var player *domain.Player

		// Try to get token from Authorization header (primary auth method)
		authHeader := c.Request().Header.Get("Authorization")
		if authHeader != "" && strings.HasPrefix(authHeader, "Bearer ") {
			token := strings.TrimPrefix(authHeader, "Bearer ")
			claims, err := ValidateToken(token)
			if err == nil {
				// Token is valid, load player from DB
				db := repository.GetDB()
				var p domain.Player
				if err := db.First(&p, "id = ?", claims.PlayerID).Error; err == nil {
					player = &p
				}
			}
		}

		// LEGACY: Fallback to player_id in query params (for development/testing only)
		// Controlled by ALLOW_LEGACY_PLAYER_ID_AUTH environment variable (default: true for backward compatibility)
		if player == nil {
			allowLegacy := true // Default to true for backward compatibility
			if legacyEnv := os.Getenv("ALLOW_LEGACY_PLAYER_ID_AUTH"); legacyEnv != "" {
				if parsed, err := strconv.ParseBool(legacyEnv); err == nil {
					allowLegacy = parsed
				}
			}

			if allowLegacy {
				playerIDStr := c.QueryParam("player_id")
				if playerIDStr != "" {
					playerID, err := uuid.Parse(playerIDStr)
					if err == nil {
						db := repository.GetDB()
						var p domain.Player
						if err := db.First(&p, "id = ?", playerID).Error; err == nil {
							player = &p
						}
					}
				}
			}
		}

		// STRICT: If no authenticated player found, return 401
		if player == nil {
			return c.JSON(401, map[string]string{"error": "Authentication required"})
		}

		// Store authenticated player in context
		c.Set(AuthContextKey, player)

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

