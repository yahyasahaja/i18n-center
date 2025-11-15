package handlers

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/your-org/i18n-center/database"
	"github.com/your-org/i18n-center/observability"
)

type HealthHandler struct{}

func NewHealthHandler() *HealthHandler {
	return &HealthHandler{}
}

// HealthCheck returns service health status
// @Summary      Health check
// @Description  Check service health and dependencies
// @Tags         health
// @Accept       json
// @Produce      json
// @Success      200  {object}  map[string]interface{}
// @Router       /health [get]
func (h *HealthHandler) HealthCheck(c *gin.Context) {
	health := map[string]interface{}{
		"status":    "healthy",
		"timestamp": time.Now().Unix(),
		"service":   "i18n-center",
		"version":   getVersion(),
	}

	// Check database connection
	dbHealthy := true
	if database.DB != nil {
		sqlDB, err := database.DB.DB()
		if err != nil {
			dbHealthy = false
		} else {
			if err := sqlDB.Ping(); err != nil {
				dbHealthy = false
			}
		}
	} else {
		dbHealthy = false
	}

	health["database"] = map[string]interface{}{
		"status": getHealthStatus(dbHealthy),
	}

	// Overall health status
	if !dbHealthy {
		health["status"] = "degraded"
		c.JSON(http.StatusServiceUnavailable, health)
		observability.RecordServiceHealth(false)
		return
	}

	observability.RecordServiceHealth(true)
	c.JSON(http.StatusOK, health)
}

// ReadinessCheck returns service readiness status
// @Summary      Readiness check
// @Description  Check if service is ready to accept traffic
// @Tags         health
// @Accept       json
// @Produce      json
// @Success      200  {object}  map[string]interface{}
// @Router       /ready [get]
func (h *HealthHandler) ReadinessCheck(c *gin.Context) {
	ready := true

	// Check database
	if database.DB != nil {
		sqlDB, err := database.DB.DB()
		if err != nil {
			ready = false
		} else {
			if err := sqlDB.Ping(); err != nil {
				ready = false
			}
		}
	} else {
		ready = false
	}

	if !ready {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"status": "not_ready",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "ready",
	})
}

// LivenessCheck returns service liveness status
// @Summary      Liveness check
// @Description  Check if service is alive
// @Tags         health
// @Accept       json
// @Produce      json
// @Success      200  {object}  map[string]interface{}
// @Router       /live [get]
func (h *HealthHandler) LivenessCheck(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status": "alive",
	})
}

func getHealthStatus(healthy bool) string {
	if healthy {
		return "healthy"
	}
	return "unhealthy"
}

func getVersion() string {
	// Could read from environment or build info
	return "1.0.0"
}

