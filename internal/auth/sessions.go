package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"clientroll/internal/database"

	"github.com/google/uuid"
)

// SessionData holds data stored in Redis for each active session.
type SessionData struct {
	UserID   string `json:"user_id"`
	TenantID string `json:"tenant_id"`
	Role     string `json:"role"`
	TokenID  string `json:"token_id"`
}

// SessionManager handles Redis-backed session management.
type SessionManager struct {
	redis           *database.Redis
	refreshDuration time.Duration
}

// NewSessionManager creates a new session manager.
func NewSessionManager(redis *database.Redis, refreshDays int) *SessionManager {
	return &SessionManager{
		redis:           redis,
		refreshDuration: time.Duration(refreshDays) * 24 * time.Hour,
	}
}

// CreateSession stores a session in Redis. Key format: session:{tenant_id}:{user_id}:{token_id}
func (s *SessionManager) CreateSession(ctx context.Context, tenantID, userID uuid.UUID, role, tokenID string) error {
	tenantKey := tenantID.String()
	// Platform-admin JWTs intentionally omit tenant_id; use a stable sentinel key
	// that matches middleware validation (`tenant_id == ""` => "platform").
	if tenantID == uuid.Nil {
		tenantKey = "platform"
	}
	data := SessionData{
		UserID:   userID.String(),
		TenantID: tenantKey,
		Role:     role,
		TokenID:  tokenID,
	}

	bytes, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("marshalling session data: %w", err)
	}

	key := s.sessionKey(tenantKey, userID.String(), tokenID)
	return s.redis.Set(ctx, key, string(bytes), s.refreshDuration)
}

// ValidateSession checks that a session exists in Redis.
func (s *SessionManager) ValidateSession(ctx context.Context, tenantID, userID, tokenID string) (*SessionData, error) {
	key := s.sessionKey(tenantID, userID, tokenID)
	val, err := s.redis.Get(ctx, key)
	if err != nil {
		return nil, fmt.Errorf("session not found or expired")
	}

	var data SessionData
	if err := json.Unmarshal([]byte(val), &data); err != nil {
		return nil, fmt.Errorf("invalid session data")
	}

	return &data, nil
}

// RevokeSession removes a specific session (logout).
func (s *SessionManager) RevokeSession(ctx context.Context, tenantID, userID, tokenID string) error {
	key := s.sessionKey(tenantID, userID, tokenID)
	return s.redis.Del(ctx, key)
}

// RevokeAllUserSessions removes all sessions for a user (password change, security event).
func (s *SessionManager) RevokeAllUserSessions(ctx context.Context, tenantID, userID string) error {
	pattern := fmt.Sprintf("session:%s:%s:*", tenantID, userID)
	return s.deleteByPattern(ctx, pattern)
}

// RevokeAllTenantSessions removes all sessions for a tenant (tenant deactivation).
func (s *SessionManager) RevokeAllTenantSessions(ctx context.Context, tenantID string) error {
	pattern := fmt.Sprintf("session:%s:*", tenantID)
	return s.deleteByPattern(ctx, pattern)
}

func (s *SessionManager) sessionKey(tenantID, userID, tokenID string) string {
	return fmt.Sprintf("session:%s:%s:%s", tenantID, userID, tokenID)
}

func (s *SessionManager) deleteByPattern(ctx context.Context, pattern string) error {
	fullPattern := s.redis.Key(pattern)
	iter := s.redis.Client.Scan(ctx, 0, fullPattern, 100).Iterator()
	for iter.Next(ctx) {
		if err := s.redis.Client.Del(ctx, iter.Val()).Err(); err != nil {
			return fmt.Errorf("deleting session: %w", err)
		}
	}
	return iter.Err()
}
