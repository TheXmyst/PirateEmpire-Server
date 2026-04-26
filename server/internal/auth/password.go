package auth

import (
	"golang.org/x/crypto/bcrypt"
)

const (
	// BcryptCost is the cost factor for bcrypt hashing
	// Using default cost (10) for balance between security and performance
	BcryptCost = bcrypt.DefaultCost
)

// HashPassword hashes a plaintext password using bcrypt
func HashPassword(plain string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(plain), BcryptCost)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}

// CheckPasswordHash compares a plaintext password with a bcrypt hash
// Returns nil if the password matches, otherwise returns an error
func CheckPasswordHash(plain, hash string) error {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(plain))
}
