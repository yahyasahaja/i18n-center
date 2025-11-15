package main

import (
	"fmt"
	"log"
	"os"

	"github.com/joho/godotenv"
	"github.com/your-org/i18n-center/auth"
	"github.com/your-org/i18n-center/database"
	"github.com/your-org/i18n-center/models"
)

func main() {
	// Load environment variables
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, using environment variables")
	}

	// Initialize database
	if err := database.InitDatabase(); err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}

	// Get admin credentials from environment or use defaults
	username := os.Getenv("ADMIN_USERNAME")
	if username == "" {
		username = "admin"
	}

	password := os.Getenv("ADMIN_PASSWORD")
	if password == "" {
		password = "admin123"
		log.Println("Warning: Using default password. Please change it!")
	}

	// Check if admin already exists
	var existingUser models.User
	result := database.DB.Where("username = ?", username).First(&existingUser)
	if result.Error == nil {
		log.Printf("User %s already exists. Skipping creation.", username)
		return
	}

	// Hash password
	hashedPassword, err := auth.HashPassword(password)
	if err != nil {
		log.Fatalf("Failed to hash password: %v", err)
	}

	// Create admin user
	admin := models.User{
		Username:     username,
		PasswordHash: hashedPassword,
		Role:         models.RoleSuperAdmin,
		IsActive:     true,
	}

	if err := database.DB.Create(&admin).Error; err != nil {
		log.Fatalf("Failed to create admin user: %v", err)
	}

	fmt.Printf("Admin user created successfully!\n")
	fmt.Printf("Username: %s\n", username)
	fmt.Printf("Password: %s\n", password)
	fmt.Printf("Role: %s\n", models.RoleSuperAdmin)
}

