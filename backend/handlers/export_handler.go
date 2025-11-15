package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/your-org/i18n-center/database"
	"github.com/your-org/i18n-center/models"
	"github.com/your-org/i18n-center/services"
)

type ExportHandler struct {
	translationService *services.TranslationService
}

func NewExportHandler() *ExportHandler {
	return &ExportHandler{
		translationService: services.NewTranslationService(),
	}
}

// ExportApplication exports all translations for an application
func (h *ExportHandler) ExportApplication(c *gin.Context) {
	applicationIDStr := c.Param("id")
	locale := c.Query("locale")
	stageStr := c.Query("stage")

	applicationID, err := uuid.Parse(applicationIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid application ID"})
		return
	}

	stage := models.DeploymentStage(stageStr)
	if stage == "" {
		stage = models.StageProduction
	}

	// Get all components for this application
	var components []models.Component
	if err := database.DB.Where("application_id = ?", applicationID).Find(&components).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	exportData := make(map[string]interface{})

	if locale != "" {
		// Export specific locale
		for _, component := range components {
			translation, err := h.translationService.GetTranslation(component.ID, locale, stage)
			if err == nil {
				exportData[component.Name] = translation.Data
			}
		}
	} else {
		// Export all locales
		exportData["components"] = make(map[string]interface{})
		for _, component := range components {
			var translations []models.TranslationVersion
			database.DB.Where("component_id = ? AND stage = ? AND is_active = ? AND version = ?",
				component.ID, stage, true, 2).Find(&translations)

			componentData := make(map[string]interface{})
			for _, trans := range translations {
				componentData[trans.Locale] = trans.Data
			}
			exportData[component.Name] = componentData
		}
	}

	c.Header("Content-Type", "application/json")
	c.Header("Content-Disposition", "attachment; filename=export.json")
	json.NewEncoder(c.Writer).Encode(exportData)
}

// ExportComponent exports translations for a specific component
// @Summary      Export component
// @Description  Export translation data for a component as JSON file
// @Tags         export
// @Accept       json
// @Produce      application/json
// @Security     BearerAuth
// @Param        id      path      string  true   "Component ID"
// @Param        locale  query     string  false  "Locale (optional, exports all if not specified)"
// @Param        stage   query     string  false  "Stage (default: production)"
// @Success      200     {file}    application/json
// @Failure      400     {object}  map[string]string
// @Failure      401     {object}  map[string]string
// @Failure      404     {object}  map[string]string
// @Router       /components/{id}/export [get]
func (h *ExportHandler) ExportComponent(c *gin.Context) {
	componentIDStr := c.Param("id")
	locale := c.Query("locale")
	stageStr := c.Query("stage")

	componentID, err := uuid.Parse(componentIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid component ID"})
		return
	}

	stage := models.DeploymentStage(stageStr)
	if stage == "" {
		stage = models.StageProduction
	}

	if locale != "" {
		// Export specific locale
		translation, err := h.translationService.GetTranslation(componentID, locale, stage)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Translation not found"})
			return
		}

		c.Header("Content-Type", "application/json")
		c.Header("Content-Disposition", "attachment; filename=component_"+locale+".json")
		json.NewEncoder(c.Writer).Encode(translation.Data)
	} else {
		// Export all locales
		var translations []models.TranslationVersion
		if err := database.DB.Where("component_id = ? AND stage = ? AND is_active = ? AND version = ?",
			componentID, stage, true, 2).Find(&translations).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		exportData := make(map[string]interface{})
		for _, trans := range translations {
			exportData[trans.Locale] = trans.Data
		}

		c.Header("Content-Type", "application/json")
		c.Header("Content-Disposition", "attachment; filename=component_all.json")
		json.NewEncoder(c.Writer).Encode(exportData)
	}
}

