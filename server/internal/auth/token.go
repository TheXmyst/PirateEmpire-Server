package auth

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"
)

var (
	// tokenSecret is loaded from environment or generated on first use
	tokenSecret []byte
)

// getTokenSecret returns the secret key for token signing
// Reads from AUTH_SECRET env var, or generates a random one (not recommended for production)
func getTokenSecret() []byte {
	if tokenSecret != nil {
		return tokenSecret
	}

	secret := os.Getenv("AUTH_SECRET")
	if secret != "" {
		tokenSecret = []byte(secret)
		return tokenSecret
	}

	// Fallback: generate a random secret (not persistent across restarts)
	// In production, AUTH_SECRET should be set via environment variable
	randomSecret := make([]byte, 32)
	if _, err := rand.Read(randomSecret); err != nil {
		// Fallback to a hardcoded secret if random generation fails (not secure, but prevents crash)
		tokenSecret = []byte("change-me-in-production-secret-key-min-32-chars")
	} else {
		tokenSecret = randomSecret
	}

	return tokenSecret
}

// TokenClaims represents the data stored in an auth token
type TokenClaims struct {
	PlayerID  uuid.UUID
	Username  string
	IsAdmin   bool
	ExpiresAt time.Time
}

// GenerateToken creates a simple signed token for a player
// Uses HMAC-SHA256 for signing
// Token format: base64(playerID|username|isAdmin|expiresAt|signature)
func GenerateToken(playerID uuid.UUID, username string, isAdmin bool) (string, error) {
	// Token expires in 24 hours
	expiresAt := time.Now().Add(24 * time.Hour)
	expiresUnix := expiresAt.Unix()

	// Create data payload
	data := fmt.Sprintf("%s|%s|%v|%d",
		playerID.String(),
		username,
		isAdmin,
		expiresUnix)

	// Create HMAC signature
	mac := hmac.New(sha256.New, getTokenSecret())
	mac.Write([]byte(data))
	signature := base64.URLEncoding.EncodeToString(mac.Sum(nil))

	// Combine data and signature
	tokenData := fmt.Sprintf("%s|%s", data, signature)

	// Base64 encode the entire token
	return base64.URLEncoding.EncodeToString([]byte(tokenData)), nil
}

// ValidateToken validates a token and returns the claims if valid
func ValidateToken(token string) (*TokenClaims, error) {
	decoded, err := base64.URLEncoding.DecodeString(token)
	if err != nil {
		return nil, fmt.Errorf("invalid token format")
	}

	parts := strings.Split(string(decoded), "|")
	if len(parts) != 5 {
		return nil, fmt.Errorf("invalid token structure")
	}

	// Reconstruct data for signature verification
	data := fmt.Sprintf("%s|%s|%s|%s", parts[0], parts[1], parts[2], parts[3])

	// Verify HMAC signature
	mac := hmac.New(sha256.New, getTokenSecret())
	mac.Write([]byte(data))
	expectedSig := base64.URLEncoding.EncodeToString(mac.Sum(nil))

	if !hmac.Equal([]byte(expectedSig), []byte(parts[4])) {
		return nil, fmt.Errorf("invalid token signature")
	}

	// Parse player ID
	playerID, err := uuid.Parse(parts[0])
	if err != nil {
		return nil, fmt.Errorf("invalid player ID in token")
	}

	// Parse expiration
	var expiresUnix int64
	if _, err := fmt.Sscanf(parts[3], "%d", &expiresUnix); err != nil {
		return nil, fmt.Errorf("invalid expiration in token")
	}
	expiresAt := time.Unix(expiresUnix, 0)

	if time.Now().After(expiresAt) {
		return nil, fmt.Errorf("token expired")
	}

	// Parse isAdmin
	var isAdmin bool
	if _, err := fmt.Sscanf(parts[2], "%v", &isAdmin); err != nil {
		return nil, fmt.Errorf("invalid admin flag in token")
	}

	claims := &TokenClaims{
		PlayerID:  playerID,
		Username:  parts[1],
		IsAdmin:   isAdmin,
		ExpiresAt: expiresAt,
	}

	return claims, nil
}

