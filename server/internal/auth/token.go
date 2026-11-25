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

// MinSecretLength est la longueur minimale de AUTH_SECRET.
const MinSecretLength = 32

var (
	// tokenSecret est la clé qui signe les jetons. Elle est chargée par
	// InitSecret au démarrage du serveur.
	tokenSecret []byte
)

// InitSecret charge la clé de signature des jetons depuis AUTH_SECRET.
//
// Sans clé fixe, n'importe quel redémarrage invaliderait tous les jetons, et
// une clé trop courte serait devinable : le serveur refuse donc de démarrer,
// sauf si devMode est vrai (développement local), auquel cas une clé
// aléatoire temporaire est générée.
func InitSecret(devMode bool) error {
	secret := os.Getenv("AUTH_SECRET")
	if secret == "" {
		if !devMode {
			return fmt.Errorf("AUTH_SECRET manquant : définis une clé d'au moins %d caractères (ou DEV_MODE=1 en local)", MinSecretLength)
		}
		tokenSecret = randomSecret()
		return nil
	}
	if len(secret) < MinSecretLength {
		return fmt.Errorf("AUTH_SECRET trop court : %d caractères, %d minimum", len(secret), MinSecretLength)
	}
	tokenSecret = []byte(secret)
	return nil
}

func randomSecret() []byte {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		// Pas de clé de secours écrite en dur : sans aléa, on s'arrête.
		panic("crypto/rand indisponible: " + err.Error())
	}
	return b
}

// getTokenSecret renvoie la clé de signature. Si InitSecret n'a pas été
// appelé (tests unitaires), elle lit AUTH_SECRET ou génère une clé aléatoire.
func getTokenSecret() []byte {
	if tokenSecret != nil {
		return tokenSecret
	}
	if secret := os.Getenv("AUTH_SECRET"); secret != "" {
		tokenSecret = []byte(secret)
	} else {
		tokenSecret = randomSecret()
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
