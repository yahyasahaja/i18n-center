package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/your-org/i18n-center/auth"
)

const (
	// CtxAPIKeyApplicationID is set when request is authenticated via application API key
	CtxAPIKeyApplicationID = "api_key_application_id"
)

// TranslationAuthMiddleware accepts either a JWT (dashboard) or an application API key (client apps).
// Sets user_id, username, role when JWT is valid; sets api_key_application_id when API key is valid.
func TranslationAuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		apiKeyHeader := c.GetHeader("X-API-Key")
		raw := ""
		if authHeader != "" {
			parts := strings.SplitN(authHeader, " ", 2)
			if len(parts) == 2 && parts[0] == "Bearer" {
				raw = strings.TrimSpace(parts[1])
			}
		}
		if raw == "" && apiKeyHeader != "" {
			raw = strings.TrimSpace(apiKeyHeader)
		}
		if raw == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Authorization header or X-API-Key required"})
			c.Abort()
			return
		}

		// If it looks like an API key (sk_...), try API key first
		if strings.HasPrefix(raw, "sk_") {
			appID, ok := auth.ValidateAPIKey(raw)
			if ok {
				c.Set(CtxAPIKeyApplicationID, appID.String())
				c.Next()
				return
			}
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid API key"})
			c.Abort()
			return
		}

		// Otherwise treat as JWT
		claims, err := auth.ValidateToken(raw)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid token"})
			c.Abort()
			return
		}
		c.Set("user_id", claims.UserID)
		c.Set("username", claims.Username)
		c.Set("role", claims.Role)
		c.Next()
	}
}

// RequireTranslationAccess allows the request if the user has one of the given roles (JWT)
// or if the request was authenticated with an application API key.
func RequireTranslationAccess(roles ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if _, hasKey := c.Get(CtxAPIKeyApplicationID); hasKey {
			c.Next()
			return
		}
		userRole, exists := c.Get("role")
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Authentication required"})
			c.Abort()
			return
		}
		roleStr := userRole.(string)
		for _, r := range roles {
			if roleStr == r {
				c.Next()
				return
			}
		}
		c.JSON(http.StatusForbidden, gin.H{"error": "Insufficient permissions"})
		c.Abort()
	}
}

// GetAPIKeyApplicationID returns the application ID when authenticated via API key, or uuid.Nil otherwise.
func GetAPIKeyApplicationID(c *gin.Context) uuid.UUID {
	v, exists := c.Get(CtxAPIKeyApplicationID)
	if !exists {
		return uuid.Nil
	}
	s, _ := v.(string)
	id, _ := uuid.Parse(s)
	return id
}
