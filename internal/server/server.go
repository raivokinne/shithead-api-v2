package server

import (
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/session"

	"api-v2/internal/database"
)

type FiberServer struct {
	*fiber.App

	db database.Service

	store *session.Store
}

func New() *FiberServer {
	store := session.New(session.Config{
		KeyLookup:      "cookie:session_id",
		Expiration:     24 * time.Hour,
		CookieSecure:   false,
		CookiePath:     "/",
		CookieSameSite: "Lax",
		CookieHTTPOnly: true,
	})

	server := &FiberServer{
		App: fiber.New(fiber.Config{
			ServerHeader: "api-v2",
			AppName:      "api-v2",
		}),

		db: database.New(),

		store: store,
	}

	return server
}
