package handlers

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/your-org/i18n-center/auth"
	"github.com/your-org/i18n-center/database"
	"github.com/your-org/i18n-center/repository"
	"github.com/your-org/i18n-center/repository/apikey"
	"github.com/your-org/i18n-center/repository/application"
	"github.com/your-org/i18n-center/services"
)

const keySegmentLen = 32 // 32 bytes = 64 hex chars after sk_

type APIKeyHandler struct {
	auditService services.AuditServicer
	keys         apikey.Repository
	apps         application.Repository
}

func NewAPIKeyHandler() *APIKeyHandler {
	return &APIKeyHandler{
		auditService: services.NewAuditService(),
		keys:         apikey.New(),
		apps:         application.New(),
	}
}

// generateKey returns a new key in the form sk_<64 hex chars>.
// Same shape as the previous GORM-era implementation — clients that stored
// a key under the old format continue to validate against the same hash.
func generateKey() (string, error) {
	b := make([]byte, keySegmentLen)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return auth.APIKeyPrefix + hex.EncodeToString(b), nil
}

// Create creates a new API key for the application. The full key is returned only in this response.
// @Summary      Create application API key
// @Description  Generate a new API key for the application. Only super_admin can create. The full key is returned once; store it securely.
// @Tags         api-keys
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id    path      string  true  "Application ID"
// @Param        body  body      object  false "Optional: { \"name\": \"My key\" }"
// @Success      201   {object}  object  "id, key (only here), key_prefix, name"
// @Failure      400   {object}  map[string]string
// @Failure      401   {object}  map[string]string
// @Failure      403   {object}  map[string]string
// @Router       /applications/{id}/api-keys [post]
func (h *APIKeyHandler) Create(c *gin.Context) {
	applicationIDStr := c.Param("id")
	applicationID, err := uuid.Parse(applicationIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid application ID"})
		return
	}

	// Verify the application exists (and is not soft-deleted) before issuing a key.
	if _, err := h.apps.GetByID(c.Request.Context(), database.SQLX, applicationID); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Application not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	var body struct {
		Name string `json:"name"`
	}
	_ = c.ShouldBindJSON(&body)

	key, err := generateKey()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate key"})
		return
	}

	prefix := key
	if len(prefix) > 12 {
		prefix = prefix[:12]
	}

	row := apikey.APIKey{
		ApplicationID: applicationID,
		KeyHash:       auth.HashKey(key),
		KeyPrefix:     prefix,
		Name:          strings.TrimSpace(body.Name),
	}
	if err := h.keys.Create(c.Request.Context(), database.SQLX, &row); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save key"})
		return
	}

	userID, username := h.getCurrentUser(c)
	ipAddress, userAgent := h.getClientInfo(c)
	h.auditService.LogCreate(userID, username, "application_api_key", row.ID, row.KeyPrefix, row, ipAddress, userAgent)

	c.JSON(http.StatusCreated, gin.H{
		"id":         row.ID,
		"key":        key,
		"key_prefix": row.KeyPrefix,
		"name":       row.Name,
		"created_at": row.CreatedAt,
	})
}

// List returns API keys for the application (key value is never returned).
// @Summary      List application API keys
// @Tags         api-keys
// @Security     BearerAuth
// @Param        id  path  string  true  "Application ID"
// @Success      200 {array} object "id, key_prefix, name, created_at"
// @Router       /applications/{id}/api-keys [get]
func (h *APIKeyHandler) List(c *gin.Context) {
	applicationIDStr := c.Param("id")
	applicationID, err := uuid.Parse(applicationIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid application ID"})
		return
	}

	keys, err := h.keys.ListByApp(c.Request.Context(), database.SQLX, applicationID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list keys"})
		return
	}
	c.JSON(http.StatusOK, keys)
}

// Delete removes an API key.
// @Summary      Delete application API key
// @Tags         api-keys
// @Security     BearerAuth
// @Param        id      path  string  true  "Application ID"
// @Param        key_id  path  string  true  "API Key ID"
// @Success      200 {object} map[string]string
// @Router       /applications/{id}/api-keys/{key_id} [delete]
func (h *APIKeyHandler) Delete(c *gin.Context) {
	applicationIDStr := c.Param("id")
	keyIDStr := c.Param("key_id")
	applicationID, err := uuid.Parse(applicationIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid application ID"})
		return
	}
	keyID, err := uuid.Parse(keyIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid key ID"})
		return
	}

	// Fetch first so we can audit-log the actual row state before deletion.
	row, err := h.keys.GetByIDForApp(c.Request.Context(), database.SQLX, keyID, applicationID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "API key not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if err := h.keys.SoftDelete(c.Request.Context(), database.SQLX, keyID, applicationID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete key"})
		return
	}

	userID, username := h.getCurrentUser(c)
	ipAddress, userAgent := h.getClientInfo(c)
	h.auditService.LogDelete(userID, username, "application_api_key", row.ID, row.KeyPrefix, row, ipAddress, userAgent)

	c.JSON(http.StatusOK, gin.H{"message": "API key deleted"})
}

func (h *APIKeyHandler) getCurrentUser(c *gin.Context) (uuid.UUID, string) {
	var userID uuid.UUID
	if v, ok := c.Get("user_id"); ok {
		if s, ok := v.(string); ok {
			userID, _ = uuid.Parse(s)
		}
	}
	var username string
	if v, ok := c.Get("username"); ok {
		if s, ok := v.(string); ok {
			username = s
		}
	}
	return userID, username
}

func (h *APIKeyHandler) getClientInfo(c *gin.Context) (string, string) {
	return c.ClientIP(), c.GetHeader("User-Agent")
}
