package server

import (
	"github.com/gofiber/contrib/websocket"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/gofiber/fiber/v2/middleware/requestid"
	"github.com/google/uuid"

	"api/internal/server/handler"
	"api/internal/server/middleware"
)

func (s *FiberServer) RegisterFiberRoutes() {
	s.App.Use(cors.New(cors.Config{
		AllowOrigins:     "https://www.troika.id.lv, http://localhost:3000",
		AllowMethods:     "GET,POST,PUT,DELETE,OPTIONS,PATCH",
		AllowHeaders:     "Accept,Authorization,Content-Type",
		AllowCredentials: true,
		MaxAge:           300,
	}))
	s.App.Use(logger.New())
	s.App.Use(recover.New())
	s.App.Use(requestid.New())
	s.store.RegisterType(uuid.New())

	authHandler := handler.NewAuthHandler(s.db, s.store)
	lobbyHandler := handler.NewLobbyHandler(s.db)
	profileHandler := handler.NewProfileHandler(s.db)
	userHandler := handler.NewUserHandler(s.db)
	notificationHandler := handler.NewNotificationHandler(s.db)
	gameHandler := handler.NewGameHandler(s.db)
	cardHandler := handler.NewCardHandler(s.db)

	s.App.Post("/register", authHandler.Register)
	s.App.Post("/login", authHandler.Login)
	s.App.Post("/logout", middleware.AuthMiddleware(s.db), authHandler.Logout)
	s.App.Get("/user", middleware.AuthMiddleware(s.db), authHandler.GetCurrentUser)
	s.App.Post("/firebase", authHandler.FirebaseLogin)

	lobbies := s.App.Group("/lobbies", middleware.AuthMiddleware(s.db))
	lobbies.Get("/", lobbyHandler.Index)
	lobbies.Post("/", lobbyHandler.Store)
	lobbies.Get("/:id/show", lobbyHandler.Show)
	lobbies.Post("/:lobbyId/join", lobbyHandler.JoinLobby)
	lobbies.Post("/:lobbyId/leave", lobbyHandler.LeaveLobby)
	lobbies.Post("/:lobbyId/invite", lobbyHandler.InviteUser)
	lobbies.Post("/invitation/accept", lobbyHandler.AcceptInvitation)
	lobbies.Post("/:lobbyId/ready", lobbyHandler.ReadyUp)

	games := s.App.Group("/games", middleware.AuthMiddleware(s.db))
	games.Use("/:gameId", func(c *fiber.Ctx) error {
		if websocket.IsWebSocketUpgrade(c) {
			c.Locals("allowed", true)
			return c.Next()
		}
		return fiber.ErrUpgradeRequired
	})
	games.Get("/:gameId", websocket.New(func(c *websocket.Conn) {
		allowed := c.Locals("allowed").(bool)
		if !allowed {
			c.Close()
			return
		}

		gameHandler.Game(c)
	}))

	cards := s.App.Group("/cards", middleware.AuthMiddleware(s.db))
	cards.Get("/:gameId/get", cardHandler.GetGameCards)

	profiles := s.App.Group("/profile", middleware.AuthMiddleware(s.db))
	profiles.Get("/:id/show", profileHandler.Show)
	profiles.Put("/:id/update", profileHandler.Update)
	profiles.Put("/:id/password", profileHandler.UpdatePassword)
	profiles.Delete("/:id/delete", profileHandler.Destroy)

	s.App.Get("/users/search", userHandler.SearchUsers)

	s.App.Get("/notifications", notificationHandler.GetNotifications)
	s.App.Put("/notifications/:id/read", notificationHandler.MarkAsRead)
	s.App.Put("/notifications/read-all", notificationHandler.MarkAllAsRead)
}
