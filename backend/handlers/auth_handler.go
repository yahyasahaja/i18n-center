package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/your-org/i18n-center/auth"
	"github.com/your-org/i18n-center/database"
	"github.com/your-org/i18n-center/models"
	"github.com/your-org/i18n-center/services"
)

type AuthHandler struct {
	auditService *services.AuditService
}

func NewAuthHandler() *AuthHandler {
	return &AuthHandler{
		auditService: services.NewAuditService(),
	}
}

// getCurrentUser extracts user info from context
func (h *AuthHandler) getCurrentUser(c *gin.Context) (userID uuid.UUID, username string) {
	userIDVal, exists := c.Get("user_id")
	if exists {
		if idStr, ok := userIDVal.(string); ok {
			if id, err := uuid.Parse(idStr); err == nil {
				userID = id
			}
		}
	}

	usernameVal, exists := c.Get("username")
	if exists {
		if name, ok := usernameVal.(string); ok {
			username = name
		}
	}

	return userID, username
}

// getClientInfo extracts IP address and user agent
func (h *AuthHandler) getClientInfo(c *gin.Context) (ipAddress, userAgent string) {
	ipAddress = c.ClientIP()
	userAgent = c.GetHeader("User-Agent")
	return ipAddress, userAgent
}

type LoginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

type LoginResponse struct {
	Token    string      `json:"token"`
	User     models.User `json:"user"`
}

// Login handles user login
// @Summary      Login
// @Description  Authenticate user and get JWT token
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        credentials  body      LoginRequest  true  "Login credentials"
// @Success      200          {object}  LoginResponse
// @Failure      400          {object}  map[string]string
// @Failure      401          {object}  map[string]string
// @Router       /auth/login [post]
func (h *AuthHandler) Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var user models.User
	if err := database.DB.Where("username = ? AND is_active = ?", req.Username, true).First(&user).Error; err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid credentials"})
		return
	}

	if !auth.CheckPasswordHash(req.Password, user.PasswordHash) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid credentials"})
		return
	}

	token, err := auth.GenerateToken(user.ID, user.Username, string(user.Role))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate token"})
		return
	}

	user.PasswordHash = "" // Don't send password hash
	c.JSON(http.StatusOK, LoginResponse{
		Token: token,
		User:  user,
	})
}

type CreateUserRequest struct {
	Username string        `json:"username" binding:"required"`
	Password string        `json:"password" binding:"required"`
	Role     models.UserRole `json:"role" binding:"required"`
}

// CreateUser creates a new user (User Manager only)
func (h *AuthHandler) CreateUser(c *gin.Context) {
	var req CreateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	hashedPassword, err := auth.HashPassword(req.Password)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to hash password"})
		return
	}

	user := models.User{
		Username:     req.Username,
		PasswordHash: hashedPassword,
		Role:         req.Role,
		IsActive:     true,
	}

	if err := database.DB.Create(&user).Error; err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Username already exists"})
		return
	}

	currentUserID, currentUsername := h.getCurrentUser(c)
	ipAddress, userAgent := h.getClientInfo(c)

	// Log audit (don't log password)
	userForAudit := models.User{
		ID:       user.ID,
		Username: user.Username,
		Role:     user.Role,
		IsActive: user.IsActive,
	}
	h.auditService.LogCreate(
		currentUserID,
		currentUsername,
		"user",
		user.ID,
		user.Username,
		userForAudit,
		ipAddress,
		userAgent,
	)

	user.PasswordHash = ""
	c.JSON(http.StatusCreated, user)
}

// GetUsers lists all users
func (h *AuthHandler) GetUsers(c *gin.Context) {
	var users []models.User
	if err := database.DB.Find(&users).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Remove password hashes
	for i := range users {
		users[i].PasswordHash = ""
	}

	c.JSON(http.StatusOK, users)
}

// UpdateUser updates user information
func (h *AuthHandler) UpdateUser(c *gin.Context) {
	userIDParam := c.Param("id")
	var user models.User

	if err := database.DB.First(&user, "id = ?", userIDParam).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	currentUserID, currentUsername := h.getCurrentUser(c)
	ipAddress, userAgent := h.getClientInfo(c)

	// Store before values for audit
	before := models.User{
		Username: user.Username,
		Role:     user.Role,
		IsActive: user.IsActive,
	}

	var req struct {
		IsActive *bool          `json:"is_active"`
		Role     *models.UserRole `json:"role"`
		Password *string        `json:"password"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.IsActive != nil {
		user.IsActive = *req.IsActive
	}
	if req.Role != nil {
		user.Role = *req.Role
	}
	if req.Password != nil {
		hashedPassword, err := auth.HashPassword(*req.Password)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to hash password"})
			return
		}
		user.PasswordHash = hashedPassword
	}

	if err := database.DB.Save(&user).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Store after values for audit
	after := models.User{
		Username: user.Username,
		Role:     user.Role,
		IsActive: user.IsActive,
	}

	// Log audit
	h.auditService.LogUpdate(
		currentUserID,
		currentUsername,
		"user",
		user.ID,
		user.Username,
		before,
		after,
		ipAddress,
		userAgent,
	)

	user.PasswordHash = ""
	c.JSON(http.StatusOK, user)
}

// GetCurrentUser returns current authenticated user
// @Summary      Get current user
// @Description  Get information about the currently authenticated user
// @Tags         auth
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Success      200  {object}  models.User
// @Failure      401  {object}  map[string]string
// @Router       /auth/me [get]
func (h *AuthHandler) GetCurrentUser(c *gin.Context) {
	userID, _ := c.Get("user_id")
	var user models.User

	if err := database.DB.First(&user, "id = ?", userID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	user.PasswordHash = ""
	c.JSON(http.StatusOK, user)
}

