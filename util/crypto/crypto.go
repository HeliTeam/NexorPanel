// Package crypto provides cryptographic utilities for password hashing and verification.
package crypto

import (
	"golang.org/x/crypto/bcrypt"
)

// BcryptCost is the bcrypt work factor for panel passwords (minimum 12 per security policy).
const BcryptCost = 12

// HashPasswordAsBcrypt generates a bcrypt hash of the given password.
func HashPasswordAsBcrypt(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), BcryptCost)
	return string(hash), err
}

// CheckPasswordHash verifies if the given password matches the bcrypt hash.
func CheckPasswordHash(hash, password string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}
