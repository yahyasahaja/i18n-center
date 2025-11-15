package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/your-org/i18n-center/models"
	"github.com/your-org/i18n-center/services"
)

type ImportHandler struct {
	translationService *services.TranslationService
}

func NewImportHandler() *ImportHandler {
	return &ImportHandler{
		translationService: services.NewTranslationService(),
	}
}

type ImportRequest struct {
	Data map[string]interface{} `json:"data" binding:"required"`
}

// ImportComponent imports translations for a component
// @Summary      Import component
// @Description  Import translation data from JSON for a component
// @Tags         import
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id      path      string            true  "Component ID"
// @Param        locale  query     string            true  "Locale"
// @Param        stage   query     string            false "Stage (default: draft)"
// @Param        request body      ImportRequest     true  "Import data"
// @Success      200     {object}  models.TranslationVersion
// @Failure      400     {object}  map[string]string
// @Failure      401     {object}  map[string]string
// @Router       /components/{id}/import [post]
func (h *ImportHandler) ImportComponent(c *gin.Context) {
	componentIDStr := c.Param("id")
	locale := c.Query("locale")
	stageStr := c.Query("stage")

	if locale == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Locale is required"})
		return
	}

	componentID, err := uuid.Parse(componentIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid component ID"})
		return
	}

	stage := models.DeploymentStage(stageStr)
	if stage == "" {
		stage = models.StageDraft
	}

	var req ImportRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Convert to JSONB
	jsonData := models.JSONB(req.Data)

	// Get user ID from context
	userIDVal, _ := c.Get("user_id")
	var userID uuid.UUID
	if idStr, ok := userIDVal.(string); ok {
		if id, err := uuid.Parse(idStr); err == nil {
			userID = id
		}
	}

	translation, err := h.translationService.SaveTranslation(componentID, locale, stage, jsonData, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, translation)
}

