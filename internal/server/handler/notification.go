package handler

import (
	"api/internal/database"
	"api/internal/database/models"
	"encoding/json"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

type NotificationHandler struct {
	db database.Service
}

type NotificationResponse struct {
	ID        uuid.UUID       `json:"id"`
	Type      string          `json:"type"`
	Data      json.RawMessage `json:"data"`
	Read      time.Time       `json:"read"`
	CreatedAt time.Time       `json:"created_at"`
}

func NewNotificationHandler(db database.Service) *NotificationHandler {
	return &NotificationHandler{
		db: db,
	}
}

func (h *NotificationHandler) GetNotifications(c *fiber.Ctx) error {
	sessionID := c.Cookies("session_id")

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

	var notifications []models.Notification
	if err := h.db.DB().Where("user_id = ?", user.ID).
		Order("created_at DESC").
		Limit(50).
		Find(&notifications).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Error fetching notifications",
		})
	}

	response := make([]NotificationResponse, len(notifications))
	for i, notif := range notifications {
		response[i] = NotificationResponse{
			ID:        notif.ID,
			Type:      *notif.Type,
			Data:      notif.Data,
			Read:      notif.ReadAt,
			CreatedAt: notif.CreatedAt,
		}
	}

	return c.JSON(response)
}

func (h *NotificationHandler) MarkAsRead(c *fiber.Ctx) error {
	notificationID := c.Params("id")
	sessionID := c.Cookies("session_id")

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

	result := h.db.DB().Model(&models.Notification{}).
		Where("id = ? AND user_id = ?", notificationID, user.ID).
		Update("read_at", time.Now())

	if result.Error != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Error marking notification as read",
		})
	}

	if result.RowsAffected == 0 {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Notification not found",
		})
	}

	return c.JSON(fiber.Map{
		"message": "Notification marked as read",
	})
}

func (h *NotificationHandler) MarkAllAsRead(c *fiber.Ctx) error {
	sessionID := c.Cookies("session_id")

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

	result := h.db.DB().Model(&models.Notification{}).
		Where("user_id = ? AND read_at IS NULL", user.ID).
		Update("read_at", time.Now())

	if result.Error != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Error marking notifications as read",
		})
	}

	return c.JSON(fiber.Map{
		"message": "All notifications marked as read",
	})
}
