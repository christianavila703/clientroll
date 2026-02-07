package auth

import (
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

// TokenType distinguishes access from refresh tokens.
type TokenType string

const (
	AccessToken  TokenType = "access"
	RefreshToken TokenType = "refresh"
)

// Claims holds the JWT claims for ClientRoll tokens.
type Claims struct {
	jwt.RegisteredClaims
	TenantID  string    `json:"tenant_id,omitempty"`
	UserID    string    `json:"user_id"`
	Role      string    `json:"role"`
	TokenType TokenType `json:"token_type"`
	TokenID   string    `json:"token_id"` // Unique ID for session tracking
}

// JWTManager handles token creation and validation.
type JWTManager struct {
	signingKey      []byte
	accessDuration  time.Duration
	refreshDuration time.Duration
}

// NewJWTManager creates a new JWT manager.
func NewJWTManager(signingKey string, accessMinutes, refreshDays int) *JWTManager {
	return &JWTManager{
		signingKey:      []byte(signingKey),
		accessDuration:  time.Duration(accessMinutes) * time.Minute,
		refreshDuration: time.Duration(refreshDays) * 24 * time.Hour,
	}
}

// GenerateTokenPair creates a new access + refresh token pair.
func (j *JWTManager) GenerateTokenPair(userID, tenantID uuid.UUID, role string) (string, string, string, error) {
	tokenID := uuid.New().String()

	accessToken, err := j.generateToken(userID, tenantID, role, AccessToken, tokenID, j.accessDuration)
	if err != nil {
		return "", "", "", fmt.Errorf("generating access token: %w", err)
	}

	refreshToken, err := j.generateToken(userID, tenantID, role, RefreshToken, tokenID, j.refreshDuration)
	if err != nil {
		return "", "", "", fmt.Errorf("generating refresh token: %w", err)
	}

	return accessToken, refreshToken, tokenID, nil
}

// GeneratePlatformTokenPair creates tokens for platform admins (no tenant_id).
func (j *JWTManager) GeneratePlatformTokenPair(adminID uuid.UUID) (string, string, string, error) {
	tokenID := uuid.New().String()

	accessToken, err := j.generateToken(adminID, uuid.Nil, "platform_admin", AccessToken, tokenID, j.accessDuration)
	if err != nil {
		return "", "", "", fmt.Errorf("generating access token: %w", err)
	}

	refreshToken, err := j.generateToken(adminID, uuid.Nil, "platform_admin", RefreshToken, tokenID, j.refreshDuration)
	if err != nil {
		return "", "", "", fmt.Errorf("generating refresh token: %w", err)
	}

	return accessToken, refreshToken, tokenID, nil
}

func (j *JWTManager) generateToken(
	userID, tenantID uuid.UUID,
	role string,
	tokenType TokenType,
	tokenID string,
	duration time.Duration,
) (string, error) {
	now := time.Now()

	claims := Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID.String(),
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(duration)),
			Issuer:    "clientroll",
		},
		UserID:    userID.String(),
		Role:      role,
		TokenType: tokenType,
		TokenID:   tokenID,
	}

	if tenantID != uuid.Nil {
		claims.TenantID = tenantID.String()
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(j.signingKey)
}

// ValidateToken parses and validates a JWT token.
func (j *JWTManager) ValidateToken(tokenString string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return j.signingKey, nil
	})

	if err != nil {
		return nil, fmt.Errorf("parsing token: %w", err)
	}

	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, fmt.Errorf("invalid token claims")
	}

	return claims, nil
}

// AccessDurationSeconds returns the access token duration in seconds.
func (j *JWTManager) AccessDurationSeconds() int {
	return int(j.accessDuration.Seconds())
}
