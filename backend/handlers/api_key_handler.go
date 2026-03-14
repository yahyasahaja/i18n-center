package handlers

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/your-org/i18n-center/database"
	"github.com/your-org/i18n-center/models"
	"github.com/your-org/i18n-center/services"
)

const keySegmentLen = 32 // 32 bytes = 64 hex chars after sk_

func hashKey(key string) string {
	h := sha256.Sum256([]byte(key))
	return hex.EncodeToString(h[:])
}

type APIKeyHandler struct {
	auditService *services.AuditService
}

func NewAPIKeyHandler() *APIKeyHandler {
	return &APIKeyHandler{auditService: services.NewAuditService()}
}

// generateKey returns a new key in the form sk_<64 hex chars>
func generateKey() (string, error) {
	b := make([]byte, keySegmentLen)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return models.APIKeyPrefix + hex.EncodeToString(b), nil
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

	var app models.Application
	if err := database.DB.First(&app, "id = ?", applicationID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Application not found"})
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

	apiKey := models.ApplicationAPIKey{
		ApplicationID: applicationID,
		KeyHash:       hashKey(key),
		KeyPrefix:     prefix,
		Name:          strings.TrimSpace(body.Name),
	}
	if err := database.DB.Create(&apiKey).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save key"})
		return
	}

	userID, username := h.getCurrentUser(c)
	ipAddress, userAgent := h.getClientInfo(c)
	h.auditService.LogCreate(userID, username, "application_api_key", apiKey.ID, apiKey.KeyPrefix, apiKey, ipAddress, userAgent)

	c.JSON(http.StatusCreated, gin.H{
		"id":         apiKey.ID,
		"key":        key,
		"key_prefix": apiKey.KeyPrefix,
		"name":      apiKey.Name,
		"created_at": apiKey.CreatedAt,
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

	var keys []models.ApplicationAPIKey
	if err := database.DB.Where("application_id = ?", applicationID).Order("created_at DESC").Find(&keys).Error; err != nil {
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

	var apiKey models.ApplicationAPIKey
	if err := database.DB.First(&apiKey, "id = ? AND application_id = ?", keyID, applicationID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "API key not found"})
		return
	}

	if err := database.DB.Delete(&apiKey).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete key"})
		return
	}

	userID, username := h.getCurrentUser(c)
	ipAddress, userAgent := h.getClientInfo(c)
	h.auditService.LogDelete(userID, username, "application_api_key", apiKey.ID, apiKey.KeyPrefix, apiKey, ipAddress, userAgent)

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
