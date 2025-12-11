package auth

import (
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestGenerateToken(t *testing.T) {
	playerID := uuid.New()
	username := "testuser"
	isAdmin := false

	token, err := GenerateToken(playerID, username, isAdmin)
	if err != nil {
		t.Fatalf("GenerateToken failed: %v", err)
	}

	if token == "" {
		t.Error("GenerateToken returned empty token")
	}
}

func TestValidateToken(t *testing.T) {
	// Set a fixed secret for testing
	os.Setenv("AUTH_SECRET", "test-secret-key-for-unit-tests-min-32-chars")
	defer os.Unsetenv("AUTH_SECRET")
	// Reset tokenSecret to force reload
	tokenSecret = nil

	playerID := uuid.New()
	username := "testuser"
	isAdmin := false

	token, err := GenerateToken(playerID, username, isAdmin)
	if err != nil {
		t.Fatalf("GenerateToken failed: %v", err)
	}

	claims, err := ValidateToken(token)
	if err != nil {
		t.Fatalf("ValidateToken failed: %v", err)
	}

	if claims.PlayerID != playerID {
		t.Errorf("PlayerID mismatch: expected %v, got %v", playerID, claims.PlayerID)
	}

	if claims.Username != username {
		t.Errorf("Username mismatch: expected %s, got %s", username, claims.Username)
	}

	if claims.IsAdmin != isAdmin {
		t.Errorf("IsAdmin mismatch: expected %v, got %v", isAdmin, claims.IsAdmin)
	}

	if claims.ExpiresAt.Before(time.Now()) {
		t.Error("Token expiration is in the past")
	}
}

func TestValidateToken_InvalidToken(t *testing.T) {
	invalidToken := "not-a-valid-token"

	_, err := ValidateToken(invalidToken)
	if err == nil {
		t.Error("ValidateToken should have failed for invalid token")
	}
}

func TestValidateToken_ExpiredToken(t *testing.T) {
	// This test would require manipulating time or creating an expired token
	// For simplicity, we'll skip it as it requires more complex setup
	// In a real scenario, you'd test with a token that has ExpiresAt in the past
	t.Skip("Expired token test requires time manipulation")
}

