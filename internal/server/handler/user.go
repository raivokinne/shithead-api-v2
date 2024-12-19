package handler

import (
	"api-v2/internal/database"
	"api-v2/internal/database/models"

	"github.com/gofiber/fiber/v2"
)

type UserHandler struct {
	db database.Service
}

type SearchUsersRequest struct {
	Query string `query:"q" validate:"required,min=2"`
}

func NewUserHandler(db database.Service) *UserHandler {
	return &UserHandler{
		db: db,
	}
}

func (h *UserHandler) SearchUsers(c *fiber.Ctx) error {
	var req SearchUsersRequest
	if err := c.QueryParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid query parameters",
		})
	}

	var users []models.User
	query := h.db.DB().
		Where("name LIKE ? OR email LIKE ?", "%"+req.Query+"%", "%"+req.Query+"%").
		Select("id, name, email, avatar").
		Limit(10)

	if err := query.Find(&users).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Error searching users",
		})
	}

	return c.JSON(users)
}
