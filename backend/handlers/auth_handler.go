package handlers

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/your-org/i18n-center/auth"
	"github.com/your-org/i18n-center/database"
	"github.com/your-org/i18n-center/repository"
	"github.com/your-org/i18n-center/repository/user"
	"github.com/your-org/i18n-center/services"
)

type AuthHandler struct {
	auditService services.AuditServicer
	users        user.Repository
}

func NewAuthHandler() *AuthHandler {
	return &AuthHandler{
		auditService: services.NewAuditService(),
		users:        user.New(),
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
	Token string    `json:"token"`
	User  user.User `json:"user"`
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

	u, err := h.users.GetActiveByUsername(c.Request.Context(), database.SQLX, req.Username)
	if err != nil {
		// Bucket all auth failures into one response — never leak which step failed.
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid credentials"})
		return
	}

	if !auth.CheckPasswordHash(req.Password, u.PasswordHash) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid credentials"})
		return
	}

	token, err := auth.GenerateToken(u.ID, u.Username, u.Role)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate token"})
		return
	}

	u.PasswordHash = "" // never serialise the hash
	c.JSON(http.StatusOK, LoginResponse{
		Token: token,
		User:  *u,
	})
}

type CreateUserRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
	Role     string `json:"role"     binding:"required"`
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

	u := user.User{
		Username:     req.Username,
		PasswordHash: hashedPassword,
		Role:         req.Role,
		IsActive:     true,
	}

	if err := h.users.Create(c.Request.Context(), database.SQLX, &u); err != nil {
		if errors.Is(err, repository.ErrConflict) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Username already exists"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	currentUserID, currentUsername := h.getCurrentUser(c)
	ipAddress, userAgent := h.getClientInfo(c)

	// Log audit — explicitly omit the password hash by reconstructing a sanitized struct.
	userForAudit := user.User{
		ID:       u.ID,
		Username: u.Username,
		Role:     u.Role,
		IsActive: u.IsActive,
	}
	h.auditService.LogCreate(
		currentUserID,
		currentUsername,
		"user",
		u.ID,
		u.Username,
		userForAudit,
		ipAddress,
		userAgent,
	)

	u.PasswordHash = ""
	c.JSON(http.StatusCreated, u)
}

// GetUsers lists all users
func (h *AuthHandler) GetUsers(c *gin.Context) {
	users, err := h.users.List(c.Request.Context(), database.SQLX)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	// Strip hashes before serialising.
	for i := range users {
		users[i].PasswordHash = ""
	}
	c.JSON(http.StatusOK, users)
}

// UpdateUser updates user information
func (h *AuthHandler) UpdateUser(c *gin.Context) {
	userIDParam := c.Param("id")
	uid, err := uuid.Parse(userIDParam)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	u, err := h.users.GetByID(c.Request.Context(), database.SQLX, uid)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	currentUserID, currentUsername := h.getCurrentUser(c)
	ipAddress, userAgent := h.getClientInfo(c)

	// Snapshot the before-state for the audit log — only the mutable, non-secret fields.
	before := user.User{
		Username: u.Username,
		Role:     u.Role,
		IsActive: u.IsActive,
	}

	var req struct {
		IsActive *bool   `json:"is_active"`
		Role     *string `json:"role"`
		Password *string `json:"password"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.IsActive != nil {
		u.IsActive = *req.IsActive
	}
	if req.Role != nil {
		u.Role = *req.Role
	}
	if req.Password != nil {
		hashedPassword, err := auth.HashPassword(*req.Password)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to hash password"})
			return
		}
		u.PasswordHash = hashedPassword
	}

	if err := h.users.Update(c.Request.Context(), database.SQLX, u); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	after := user.User{
		Username: u.Username,
		Role:     u.Role,
		IsActive: u.IsActive,
	}

	h.auditService.LogUpdate(
		currentUserID,
		currentUsername,
		"user",
		u.ID,
		u.Username,
		before,
		after,
		ipAddress,
		userAgent,
	)

	u.PasswordHash = ""
	c.JSON(http.StatusOK, u)
}

// GetCurrentUser returns current authenticated user
// @Summary      Get current user
// @Description  Get information about the currently authenticated user
// @Tags         auth
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Success      200  {object}  user.User
// @Failure      401  {object}  map[string]string
// @Router       /auth/me [get]
func (h *AuthHandler) GetCurrentUser(c *gin.Context) {
	// The auth middleware stores user_id as uuid.UUID, not as a string.
	// Accept either form so a future middleware that stringifies the claim
	// doesn't silently 401 every request.
	v, ok := c.Get("user_id")
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Missing user context"})
		return
	}
	var uid uuid.UUID
	switch typed := v.(type) {
	case uuid.UUID:
		uid = typed
	case string:
		parsed, err := uuid.Parse(typed)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid user context"})
			return
		}
		uid = parsed
	default:
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid user context"})
		return
	}

	u, err := h.users.GetByID(c.Request.Context(), database.SQLX, uid)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	u.PasswordHash = ""
	c.JSON(http.StatusOK, u)
}
