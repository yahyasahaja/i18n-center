package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"

	"github.com/joho/godotenv"

	"github.com/your-org/i18n-center/auth"
	"github.com/your-org/i18n-center/database"
	"github.com/your-org/i18n-center/repository"
	"github.com/your-org/i18n-center/repository/user"
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

	ctx := context.Background()
	users := user.New()

	// Check if admin already exists
	if existing, err := users.GetActiveByUsername(ctx, database.SQLX, username); err == nil && existing != nil {
		log.Printf("User %s already exists. Skipping creation.", username)
		return
	} else if err != nil && !errors.Is(err, repository.ErrNotFound) {
		log.Fatalf("Failed to look up user: %v", err)
	}

	// Hash password
	hashedPassword, err := auth.HashPassword(password)
	if err != nil {
		log.Fatalf("Failed to hash password: %v", err)
	}

	// Create admin user
	admin := &user.User{
		Username:     username,
		PasswordHash: hashedPassword,
		Role:         user.RoleSuperAdmin,
		IsActive:     true,
	}

	if err := users.Create(ctx, database.SQLX, admin); err != nil {
		log.Fatalf("Failed to create admin user: %v", err)
	}

	fmt.Printf("Admin user created successfully!\n")
	fmt.Printf("Username: %s\n", username)
	fmt.Printf("Password: %s\n", password)
	fmt.Printf("Role: %s\n", user.RoleSuperAdmin)
}
