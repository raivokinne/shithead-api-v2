package handler

import (
	"api-v2/internal/database"
	"api-v2/internal/database/models"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

type NotificationHandler struct {
	db database.Service
}

type NotificationResponse struct {
	ID        uuid.UUID      `json:"id"`
	Type      string    `json:"type"`
	Data      string `json:"data"`
	Read      time.Time      `json:"read"`
}

func NewNotificationHandler(db database.Service) *NotificationHandler {
	return &NotificationHandler{
		db: db,
	}
}

func (h *NotificationHandler) GetNotifications(c *fiber.Ctx) error {
	userID := c.Locals("user_id").(uuid.UUID)
	if userID == uuid.Nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "Unauthorized",
		})
	}

	var notifications []models.Notification
	if err := h.db.DB().Where("user_id = ?", userID).
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
		}
	}

	return c.JSON(response)
}

func (h *NotificationHandler) MarkAsRead(c *fiber.Ctx) error {
	notificationID := c.Params("id")
	userID := c.Locals("user_id").(uuid.UUID)

	result := h.db.DB().Model(&models.Notification{}).
		Where("id = ? AND user_id = ?", notificationID, userID).
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
	userID := c.Locals("user_id").(uint)

	result := h.db.DB().Model(&models.Notification{}).
		Where("user_id = ? AND read_at IS NULL", userID).
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
