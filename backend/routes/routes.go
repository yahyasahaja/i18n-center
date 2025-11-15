package routes

import (
	"os"

	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
	"github.com/your-org/i18n-center/handlers"
	"github.com/your-org/i18n-center/middleware"
)

func SetupRoutes() *gin.Engine {
	r := gin.Default()

	// Observability middleware (must be first)
	r.Use(middleware.PanicRecoveryMiddleware())
	r.Use(middleware.ObservabilityMiddleware())
	r.Use(middleware.ErrorLoggingMiddleware())

	// CORS middleware
	r.Use(func(c *gin.Context) {
		corsOrigin := os.Getenv("CORS_ORIGIN")
		if corsOrigin == "" {
			corsOrigin = "*"
		}
		c.Writer.Header().Set("Access-Control-Allow-Origin", corsOrigin)
		c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, accept, origin, Cache-Control, X-Requested-With")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS, GET, PUT, DELETE, PATCH")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	})

	// Initialize handlers
	healthHandler := handlers.NewHealthHandler()
	authHandler := handlers.NewAuthHandler()
	appHandler := handlers.NewApplicationHandler()
	componentHandler := handlers.NewComponentHandler()
	translationHandler := handlers.NewTranslationHandler()
	exportHandler := handlers.NewExportHandler()
	importHandler := handlers.NewImportHandler()
	auditHandler := handlers.NewAuditHandler()

	// Swagger documentation
	// Accessible at: http://localhost:8080/api/docs/index.html
	r.GET("/api/docs/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))
	// Also available at root for convenience
	r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	// Health check routes (public, no auth required)
	r.GET("/health", healthHandler.HealthCheck)
	r.GET("/ready", healthHandler.ReadinessCheck)
	r.GET("/live", healthHandler.LivenessCheck)

	// Public routes
	r.POST("/api/auth/login", authHandler.Login)

	// Protected routes
	api := r.Group("/api")
	api.Use(middleware.AuthMiddleware())

	// Auth routes
	api.GET("/auth/me", authHandler.GetCurrentUser)
	api.GET("/auth/users", authHandler.GetUsers, middleware.RequireRole("super_admin", "user_manager"))
	api.POST("/auth/users", authHandler.CreateUser, middleware.RequireRole("super_admin", "user_manager"))
	api.PUT("/auth/users/:id", authHandler.UpdateUser, middleware.RequireRole("super_admin", "user_manager"))

	// Application routes
	api.GET("/applications", appHandler.GetApplications, middleware.RequireRole("super_admin", "operator"))
	api.GET("/applications/:id", appHandler.GetApplication, middleware.RequireRole("super_admin", "operator"))
	api.POST("/applications", appHandler.CreateApplication, middleware.RequireRole("super_admin", "operator"))
	api.PUT("/applications/:id", appHandler.UpdateApplication, middleware.RequireRole("super_admin", "operator"))
	api.DELETE("/applications/:id", appHandler.DeleteApplication, middleware.RequireRole("super_admin"))

	// Translation routes (must come before component routes to avoid conflict)
	// Bulk/aggregator endpoint (must come before single component routes)
	api.GET("/translations/bulk", translationHandler.GetMultipleTranslations, middleware.RequireRole("super_admin", "operator"))

	translations := api.Group("/components/:id")
	translations.GET("/translations", translationHandler.GetTranslation, middleware.RequireRole("super_admin", "operator"))
	translations.POST("/translations", translationHandler.SaveTranslation, middleware.RequireRole("super_admin", "operator"))
	translations.POST("/translations/revert", translationHandler.RevertTranslation, middleware.RequireRole("super_admin", "operator"))
	translations.POST("/translations/deploy", translationHandler.DeployTranslation, middleware.RequireRole("super_admin", "operator"))
	translations.POST("/translations/auto-translate", translationHandler.AutoTranslate, middleware.RequireRole("super_admin", "operator"))
	translations.POST("/translations/backfill", translationHandler.BackfillTranslations, middleware.RequireRole("super_admin", "operator"))
	translations.GET("/translations/compare", translationHandler.GetVersionComparison, middleware.RequireRole("super_admin", "operator"))

	// Component routes
	api.GET("/components", componentHandler.GetComponents, middleware.RequireRole("super_admin", "operator"))
	api.GET("/components/:id", componentHandler.GetComponent, middleware.RequireRole("super_admin", "operator"))
	api.POST("/components", componentHandler.CreateComponent, middleware.RequireRole("super_admin", "operator"))
	api.PUT("/components/:id", componentHandler.UpdateComponent, middleware.RequireRole("super_admin", "operator"))
	api.DELETE("/components/:id", componentHandler.DeleteComponent, middleware.RequireRole("super_admin", "operator"))

	// Export/Import routes
	api.GET("/applications/:id/export", exportHandler.ExportApplication, middleware.RequireRole("super_admin", "operator"))
	api.GET("/components/:id/export", exportHandler.ExportComponent, middleware.RequireRole("super_admin", "operator"))
	api.POST("/components/:id/import", importHandler.ImportComponent, middleware.RequireRole("super_admin", "operator"))

	// Audit routes
	api.GET("/audit/logs", auditHandler.GetAuditLogs, middleware.RequireRole("super_admin", "operator"))
	api.GET("/audit/history/:resource_type/:resource_id", auditHandler.GetResourceHistory, middleware.RequireRole("super_admin", "operator"))

	return r
}

