package middleware

import (
	"api/internal/database"
	"api/internal/database/models"
	"time"

	"github.com/gofiber/fiber/v2"
)

func AuthMiddleware(db database.Service) fiber.Handler {
    return func(c *fiber.Ctx) error {
        sessionID := c.Cookies("session_id")
        if sessionID == "" {
            return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
                "error": "Session ID is missing",
            })
        }

        var session models.Session
        if err := db.DB().Where("id = ?", sessionID).First(&session).Error; err != nil {
            return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
                "error": "Invalid session",
            })
        }

        currentTime := int(time.Now().Unix())
        if session.LastActivity + (24 * 3600) < currentTime {
            return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
                "error": "Session expired",
            })
        }

        c.Locals("user_id", session.UserID)
        c.Locals("session_id", session.ID)
        return c.Next()
    }
}

