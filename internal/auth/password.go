package auth

import (
	"fmt"
	"strings"
	"unicode"

	"golang.org/x/crypto/bcrypt"
)

const (
	// bcryptCost is the cost parameter for bcrypt hashing (12 = ~250ms).
	bcryptCost = 12

	// minPasswordLength is the minimum password length.
	minPasswordLength = 10
)

// HashPassword hashes a password using bcrypt.
func HashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), bcryptCost)
	if err != nil {
		return "", fmt.Errorf("hashing password: %w", err)
	}
	return string(bytes), nil
}

// CheckPassword verifies a password against a bcrypt hash.
func CheckPassword(password, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}

// ValidatePasswordStrength checks that a password meets complexity requirements.
// All violations are returned in a single error message.
func ValidatePasswordStrength(password string) error {
	var problems []string

	if len(password) < minPasswordLength {
		problems = append(problems, fmt.Sprintf("at least %d characters", minPasswordLength))
	}

	var hasUpper, hasLower, hasDigit bool
	for _, ch := range password {
		switch {
		case unicode.IsUpper(ch):
			hasUpper = true
		case unicode.IsLower(ch):
			hasLower = true
		case unicode.IsDigit(ch):
			hasDigit = true
		}
	}

	if !hasUpper {
		problems = append(problems, "an uppercase letter")
	}
	if !hasLower {
		problems = append(problems, "a lowercase letter")
	}
	if !hasDigit {
		problems = append(problems, "a digit")
	}

	if len(problems) > 0 {
		return fmt.Errorf("password must contain %s", strings.Join(problems, ", "))
	}

	return nil
}
