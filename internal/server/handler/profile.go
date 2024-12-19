package handler

import (
	"api-v2/internal/database"
	"api-v2/internal/database/models"
	"errors"
	"fmt"
	"mime/multipart"
	"os"
	"path/filepath"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

type ProfileHandler struct {
	db database.Service
}

type UpdateProfileRequest struct {
	Name  string                `form:"name" validate:"required,max=255"`
	Email string                `form:"email" validate:"required,email"`
	Avatar *multipart.FileHeader `form:"avatar"`
}

type UpdatePasswordRequest struct {
	CurrentPassword string `json:"current_password" validate:"required"`
	NewPassword     string `json:"new_password" validate:"required,min=8"`
	ConfirmPassword string `json:"new_password_confirmation" validate:"required,min=8"`
}

func NewProfileHandler(db database.Service) *ProfileHandler {
	return &ProfileHandler{
		db: db,
	}
}

func (h *ProfileHandler) Show(c *fiber.Ctx) error {
	id := c.Params("id")
	var user models.User

	if err := h.db.DB().First(&user, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"error": "User not found",
			})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Database error",
		})
	}

	return c.JSON(user)
}

func (h *ProfileHandler) Update(c *fiber.Ctx) error {
	id := c.Params("id")
	var user models.User
	if err := h.db.DB().First(&user, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"error": "User not found",
			})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Database error",
		})
	}

	var req UpdateProfileRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	var existingUser models.User
	result := h.db.DB().Where("email = ? AND id != ?", req.Email, id).First(&existingUser)
	if result.Error == nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Email already in use",
		})
	}

	if file, err := c.FormFile("avatar"); err == nil {
		ext := strings.ToLower(filepath.Ext(file.Filename))
		if !isValidImageExt(ext) {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "Invalid file type. Allowed types: jpeg, png, jpg, gif",
			})
		}

		filename := fmt.Sprintf("avatars/%s%s", uuid.New().String(), ext)

		if err := c.SaveFile(file, fmt.Sprintf("./public/%s", filename)); err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "Error saving file",
			})
		}

		if *user.Avatar != "" {
			if err := os.Remove(fmt.Sprintf("./public/%s", *user.Avatar)); err != nil {
				fmt.Printf("Error deleting old avatar: %v\n", err)
			}
		}

		*user.Avatar = filename
	}

	user.Name = req.Name
	user.Email = req.Email

	if err := h.db.DB().Save(&user).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Error updating user",
		})
	}

	return c.JSON(fiber.Map{
		"success": true,
	})
}

func (h *ProfileHandler) UpdatePassword(c *fiber.Ctx) error {
	id := c.Params("id")
	var user models.User
	if err := h.db.DB().First(&user, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"error": "User not found",
			})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Database error",
		})
	}

	var req UpdatePasswordRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	if req.NewPassword != req.ConfirmPassword {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Passwords do not match",
		})
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.CurrentPassword)); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Current password is incorrect",
		})
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Error hashing password",
		})
	}

	user.Password = string(hashedPassword)
	if err := h.db.DB().Save(&user).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Error updating password",
		})
	}

	return c.JSON(fiber.Map{
		"success": true,
	})
}

func (h *ProfileHandler) Destroy(c *fiber.Ctx) error {
	id := c.Params("id")
	var user models.User
	if err := h.db.DB().First(&user, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"error": "User not found",
			})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Database error",
		})
	}

	if *user.Avatar != "" {
		if err := os.Remove(fmt.Sprintf("./public/%s", *user.Avatar)); err != nil {
			fmt.Printf("Error deleting avatar: %v\n", err)
		}
	}

	if err := h.db.DB().Delete(&user).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Error deleting user",
		})
	}

	return c.JSON(fiber.Map{
		"success": true,
	})
}

func isValidImageExt(ext string) bool {
	validExts := map[string]bool{
		".jpg":  true,
		".jpeg": true,
		".png":  true,
		".gif":  true,
	}
	return validExts[ext]
}
