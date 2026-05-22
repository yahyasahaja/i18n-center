package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/your-org/i18n-center/database"
	"github.com/your-org/i18n-center/repository/component"
	"github.com/your-org/i18n-center/repository/translation"
	"github.com/your-org/i18n-center/services"
)

type ExportHandler struct {
	translationService *services.TranslationService
	components         component.Repository
	translations       translation.Repository
}

func NewExportHandler() *ExportHandler {
	return &ExportHandler{
		translationService: services.NewTranslationService(),
		components:         component.New(),
		translations:       translation.New(),
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

	stage := translation.Stage(stageStr)
	if stage == "" {
		stage = translation.StageProduction
	}

	ctx := c.Request.Context()
	components, _, err := h.components.List(ctx, database.SQLX, component.ListFilter{ApplicationID: applicationID})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	exportData := make(map[string]interface{})

	if locale != "" {
		// Export specific locale — one entry per component.
		for _, comp := range components {
			v, err := h.translationService.GetTranslation(comp.ID, locale, stage)
			if err == nil {
				exportData[comp.Name] = v.Data
			}
		}
	} else {
		// Export all locales — per component, one entry per locale.
		for _, comp := range components {
			versions, err := h.translations.ListLatestLocales(ctx, database.SQLX, comp.ID, stage)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			componentData := make(map[string]interface{}, len(versions))
			for _, v := range versions {
				componentData[v.Locale] = v.Data
			}
			exportData[comp.Name] = componentData
		}
	}

	c.Header("Content-Type", "application/json")
	c.Header("Content-Disposition", "attachment; filename=export.json")
	_ = json.NewEncoder(c.Writer).Encode(exportData)
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

	stage := translation.Stage(stageStr)
	if stage == "" {
		stage = translation.StageProduction
	}

	ctx := c.Request.Context()
	if locale != "" {
		// Export specific locale
		v, err := h.translationService.GetTranslation(componentID, locale, stage)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Translation not found"})
			return
		}

		c.Header("Content-Type", "application/json")
		c.Header("Content-Disposition", "attachment; filename=component_"+locale+".json")
		_ = json.NewEncoder(c.Writer).Encode(v.Data)
		return
	}

	// Export all locales — one entry per locale at the requested stage.
	versions, err := h.translations.ListLatestLocales(ctx, database.SQLX, componentID, stage)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	exportData := make(map[string]interface{}, len(versions))
	for _, v := range versions {
		exportData[v.Locale] = v.Data
	}

	c.Header("Content-Type", "application/json")
	c.Header("Content-Disposition", "attachment; filename=component_all.json")
	_ = json.NewEncoder(c.Writer).Encode(exportData)
}
