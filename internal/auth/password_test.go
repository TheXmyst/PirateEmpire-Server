package auth

import (
	"testing"
)

func TestHashPassword(t *testing.T) {
	password := "testpassword123"
	hash, err := HashPassword(password)
	if err != nil {
		t.Fatalf("HashPassword failed: %v", err)
	}

	if hash == "" {
		t.Error("HashPassword returned empty string")
	}

	if hash == password {
		t.Error("HashPassword returned plaintext password")
	}

	// Hash should be different each time (due to salt)
	hash2, err := HashPassword(password)
	if err != nil {
		t.Fatalf("HashPassword failed on second call: %v", err)
	}

	if hash == hash2 {
		t.Error("HashPassword returned same hash for same password (should be different due to salt)")
	}
}

func TestCheckPasswordHash(t *testing.T) {
	password := "testpassword123"
	hash, err := HashPassword(password)
	if err != nil {
		t.Fatalf("HashPassword failed: %v", err)
	}

	// Correct password should match
	err = CheckPasswordHash(password, hash)
	if err != nil {
		t.Errorf("CheckPasswordHash failed for correct password: %v", err)
	}

	// Wrong password should not match
	wrongPassword := "wrongpassword"
	err = CheckPasswordHash(wrongPassword, hash)
	if err == nil {
		t.Error("CheckPasswordHash should have failed for wrong password")
	}
}

func TestCheckPasswordHash_InvalidHash(t *testing.T) {
	password := "testpassword123"
	invalidHash := "not-a-valid-bcrypt-hash"

	err := CheckPasswordHash(password, invalidHash)
	if err == nil {
		t.Error("CheckPasswordHash should have failed for invalid hash")
	}
}

