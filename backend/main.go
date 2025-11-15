package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"go.uber.org/zap"

	"github.com/joho/godotenv"
	"github.com/your-org/i18n-center/cache"
	"github.com/your-org/i18n-center/database"
	"github.com/your-org/i18n-center/observability"
	"github.com/your-org/i18n-center/routes"

	_ "github.com/your-org/i18n-center/docs" // Swagger docs
)

// @title           i18n Center API
// @version         1.0
// @description     Centralized i18n management service API
// @termsOfService  http://swagger.io/terms/

// @contact.name   API Support
// @contact.url    http://www.swagger.io/support
// @contact.email  support@swagger.io

// @license.name  Apache 2.0
// @license.url   http://www.apache.org/licenses/LICENSE-2.0.html

// @host      localhost:8080
// @BasePath  /api

// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
// @description Type "Bearer" followed by a space and JWT token.

func main() {
	// Load environment variables
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, using environment variables")
	}

	// Initialize observability first
	if err := observability.InitLogger(); err != nil {
		log.Fatalf("Failed to initialize logger: %v", err)
	}
	defer observability.Logger.Sync()

	// Initialize metrics (optional - service works without it)
	if err := observability.InitMetrics(); err != nil {
		observability.Logger.Warn("Failed to initialize metrics (continuing without Datadog)", zap.Error(err))
	} else if observability.StatsdClient != nil {
		observability.Logger.Info("Datadog metrics initialized")
	} else {
		observability.Logger.Info("Datadog metrics disabled (DD_ENABLED=false or not set)")
	}

	// Initialize tracing (optional - service works without it)
	if err := observability.InitTracing(); err != nil {
		observability.Logger.Warn("Failed to initialize tracing (continuing without Datadog)", zap.Error(err))
	} else if observability.IsTracingEnabled() {
		observability.Logger.Info("Datadog tracing initialized")
		defer observability.StopTracing()
	} else {
		observability.Logger.Info("Datadog tracing disabled (DD_ENABLED=false or not set)")
	}

	observability.Logger.Info("Initializing i18n-center service")

	// Initialize database
	if err := database.InitDatabase(); err != nil {
		observability.Logger.Fatal("Failed to initialize database", zap.Error(err))
	}

	// Initialize Redis cache
	if err := cache.InitCache(); err != nil {
		observability.Logger.Warn("Failed to initialize Redis cache", zap.Error(err))
		observability.Logger.Info("Continuing without cache...")
	}

	// Setup routes
	r := routes.SetupRoutes()

	// Setup graceful shutdown
	setupGracefulShutdown()

	// Get port from environment
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	observability.Logger.Info("Server starting", zap.String("port", port))
	observability.RecordServiceHealth(true)

	if err := r.Run(fmt.Sprintf(":%s", port)); err != nil {
		observability.Logger.Fatal("Failed to start server", zap.Error(err))
	}
}

func setupGracefulShutdown() {
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-c
		observability.Logger.Info("Shutting down gracefully...")
		observability.RecordServiceHealth(false)
		observability.Logger.Sync()
		observability.StopTracing()
		os.Exit(0)
	}()
}
