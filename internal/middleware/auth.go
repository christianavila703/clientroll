package middleware

import (
	"context"
	"net/http"
	"strings"

	"clientroll/internal/auth"
	"clientroll/internal/models"
)

type contextKey string

const (
	CtxUserID   contextKey = "user_id"
	CtxTenantID contextKey = "tenant_id"
	CtxRole     contextKey = "role"
	CtxTokenID  contextKey = "token_id"
)

// AuthMiddleware validates JWT tokens and populates request context.
type AuthMiddleware struct {
	jwtManager     *auth.JWTManager
	sessionManager *auth.SessionManager
}

// NewAuthMiddleware creates a new auth middleware.
func NewAuthMiddleware(jm *auth.JWTManager, sm *auth.SessionManager) *AuthMiddleware {
	return &AuthMiddleware{
		jwtManager:     jm,
		sessionManager: sm,
	}
}

// RequireAuth ensures the request has a valid access token with an active session.
func (a *AuthMiddleware) RequireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		claims, err := a.extractAndValidateToken(r)
		if err != nil {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "Unauthorised"})
			return
		}

		if claims.TokenType != auth.AccessToken {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "Invalid token type"})
			return
		}

		// Verify session exists in Redis (enables instant revocation)
		tenantID := claims.TenantID
		if tenantID == "" {
			tenantID = "platform"
		}
		_, err = a.sessionManager.ValidateSession(r.Context(), tenantID, claims.UserID, claims.TokenID)
		if err != nil {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "Session expired"})
			return
		}

		// Populate context -- all downstream code reads from here, NEVER from request params
		ctx := r.Context()
		ctx = context.WithValue(ctx, CtxUserID, claims.UserID)
		ctx = context.WithValue(ctx, CtxTenantID, claims.TenantID)
		ctx = context.WithValue(ctx, CtxRole, claims.Role)
		ctx = context.WithValue(ctx, CtxTokenID, claims.TokenID)

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// RequireRole creates middleware that checks the user has one of the specified roles.
func (a *AuthMiddleware) RequireRole(roles ...string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			role := GetRole(r.Context())
			for _, allowed := range roles {
				if role == allowed {
					next.ServeHTTP(w, r)
					return
				}
			}
			writeJSON(w, http.StatusForbidden, map[string]string{"error": "Insufficient permissions"})
		})
	}
}

// RequirePlatformAdmin ensures the user is a platform admin.
func (a *AuthMiddleware) RequirePlatformAdmin(next http.Handler) http.Handler {
	return a.RequireRole(models.RolePlatformAdmin)(next)
}

// RequireTenantAdmin ensures the user is a tenant admin.
func (a *AuthMiddleware) RequireTenantAdmin(next http.Handler) http.Handler {
	return a.RequireRole(models.RoleTenantAdmin)(next)
}

func (a *AuthMiddleware) extractAndValidateToken(r *http.Request) (*auth.Claims, error) {
	header := r.Header.Get("Authorization")
	if header == "" {
		return nil, ErrNoToken
	}

	parts := strings.SplitN(header, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "bearer") {
		return nil, ErrInvalidFormat
	}

	return a.jwtManager.ValidateToken(parts[1])
}

// Context helpers -- extract values set by auth middleware.

// GetUserID returns the user ID from context.
func GetUserID(ctx context.Context) string {
	if v, ok := ctx.Value(CtxUserID).(string); ok {
		return v
	}
	return ""
}

// GetTenantID returns the tenant ID from context.
func GetTenantID(ctx context.Context) string {
	if v, ok := ctx.Value(CtxTenantID).(string); ok {
		return v
	}
	return ""
}

// GetRole returns the role from context.
func GetRole(ctx context.Context) string {
	if v, ok := ctx.Value(CtxRole).(string); ok {
		return v
	}
	return ""
}

// GetTokenID returns the token ID from context.
func GetTokenID(ctx context.Context) string {
	if v, ok := ctx.Value(CtxTokenID).(string); ok {
		return v
	}
	return ""
}
