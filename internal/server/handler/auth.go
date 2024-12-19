package handler

import (
	"errors"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/session"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"

	"api-v2/internal/database"
	"api-v2/internal/database/models"
	"api-v2/internal/server/utils"
)

type AuthHandler struct {
	store *session.Store
	db    database.Service
}

type LoginRequest struct {
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required,min=6"`
}

type RegisterRequest struct {
	Name                 string `json:"name" validate:"required"`
	Email                string `json:"email" validate:"required,email"`
	Password             string `json:"password" validate:"required,min=6"`
	PasswordConfirmation string `json:"password_confirmation" validate:"required,min=6"`
}

func NewAuthHandler(db database.Service, store *session.Store) *AuthHandler {
	return &AuthHandler{
		store: store,
		db:    db,
	}
}

func (h *AuthHandler) Register(c *fiber.Ctx) error {
	var req RegisterRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	var existingUser models.User
	result := h.db.DB().Where("email = ?", req.Email).First(&existingUser)
	if result.Error == nil {
		return c.Status(fiber.StatusConflict).JSON(fiber.Map{
			"error": "User already exists",
		})
	} else if !errors.Is(result.Error, gorm.ErrRecordNotFound) {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Database error",
		})
	}

	if req.Password != req.PasswordConfirmation {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Passwords do not match",
		})
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Error hashing password",
		})
	}

	user := models.User{
		ID:       uuid.New(),
		Name:     req.Name,
		Email:    req.Email,
		Password: string(hashedPassword),
	}

	if err := h.db.DB().Create(&user).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Error creating user",
		})
	}

	lastUsedAt := time.Now()

	token := models.PersonalAccessToken{
		ID:            uuid.New(),
		TokenableType: "User",
		TokenableID:   user.ID,
		Name:          "Primary",
		Token:         utils.GenerateToken(),
		Abilities:     nil,
		LastUsedAt:    &lastUsedAt,
	}

	if err := h.db.DB().Create(&token).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Error creating token",
		})
	}

	sess, err := h.store.Get(c)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Session error",
		})
	}

	sess.Set("user_id", user.ID)
	if err := sess.Save(); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Error saving session",
		})
	}

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"success": true,
		"message": "User created successfully",
		"token":   token.Token,
	})
}

func (h *AuthHandler) Login(c *fiber.Ctx) error {
	var req LoginRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	var user models.User
	result := h.db.DB().Where("email = ?", req.Email).First(&user)
	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "Invalid credentials",
			})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Database error",
		})
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.Password)); err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "Invalid credentials",
		})
	}

	sessionID := c.Cookies("session_id")
	if sessionID != "" {
		var session models.Session
		if err := h.db.DB().Where("id = ?", sessionID).First(&session).Error; err == nil {
			return c.JSON(fiber.Map{
				"message":    "Already logged in",
				"session_id": session.ID,
			})
		}
	}

	sess, err := h.store.Get(c)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Session error",
		})
	}

	sess.SetExpiry(time.Hour * 24)
	sess.Set("user_id", user.ID)

	session := models.Session{
		ID:           uuid.New(),
		UserID:       user.ID,
		IPAddress:    c.IP(),
		UserAgent:    c.Get("User-Agent"),
		LastActivity: int(time.Now().Unix()),
	}

	if err := h.db.DB().Create(&session).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Error creating session",
		})
	}

	sess.Set("session_id", session.ID)
	if err := sess.Save(); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Error saving session",
		})
	}

	c.Cookie(&fiber.Cookie{
		Name:     "session_id",
		Value:    session.ID.String(),
		Expires:  time.Now().Add(24 * time.Hour),
		HTTPOnly: true,
		Secure:   true,
		SameSite: "Lax",
	})

	var token models.PersonalAccessToken

	if err := h.db.DB().Where("tokenable_type = ? AND tokenable_id = ?", "User", user.ID).First(&token).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Error creating token",
		})
	}

	return c.JSON(fiber.Map{
		"success": true,
		"message": "Login successful",
		"token":   token.Token,
	})
}

func (h *AuthHandler) Logout(c *fiber.Ctx) error {
	sessionID := c.Cookies("session_id")
	if sessionID == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "Session ID not provided",
		})
	}

	var session models.Session
	if err := h.db.DB().Where("id = ?", sessionID).First(&session).Error; err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "Invalid session",
		})
	}

	if err := h.db.DB().Delete(&session).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Error logging out. Unable to delete session",
		})
	}

	c.ClearCookie("session_id")

	return c.JSON(fiber.Map{
		"message": "Successfully logged out",
	})
}

func (h *AuthHandler) GetCurrentUser(c *fiber.Ctx) error {
	sessionID := c.Cookies("session_id")
	if sessionID == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "Session ID not provided",
		})
	}

	var session models.Session
	if err := h.db.DB().Where("id = ?", sessionID).First(&session).Error; err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "Invalid session",
		})
	}

	var user models.User
	if err := h.db.DB().First(&user, session.UserID).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Error fetching user",
		})
	}

	return c.JSON(user)
}
