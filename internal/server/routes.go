package server

import (
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/google/uuid"

	"api-v2/internal/server/handler"
	"api-v2/internal/server/middleware"
)

func (s *FiberServer) RegisterFiberRoutes() {
	s.App.Use(cors.New(cors.Config{
		AllowOrigins:     "https://www.troika.id.lv",
		// AllowOrigins:     "http://localhost:3001",
		AllowMethods:     "GET,POST,PUT,DELETE,OPTIONS,PATCH",
		AllowHeaders:     "Accept,Authorization,Content-Type",
		AllowCredentials: true,
		MaxAge:           300,
	}))
	s.App.Use(recover.New())
	s.App.Use(logger.New())
	s.store.RegisterType(uuid.New())

	authHandler := handler.NewAuthHandler(s.db, s.store)
	lobbyHandler := handler.NewLobbyHandler(s.db)
	profileHandler := handler.NewProfileHandler(s.db)
	userHandler := handler.NewUserHandler(s.db)
	notificationHandler := handler.NewNotificationHandler(s.db)

	api := s.App.Group("/api")

	api.Post("/register", authHandler.Register)
	api.Post("/login", authHandler.Login)
	api.Post("/logout", middleware.AuthMiddleware(s.db), authHandler.Logout)
	api.Get("/user", middleware.AuthMiddleware(s.db), authHandler.GetCurrentUser)

	lobbies := api.Group("/lobbies", middleware.AuthMiddleware(s.db))
	lobbies.Get("/", lobbyHandler.Index)
	lobbies.Post("/", lobbyHandler.Store)
	lobbies.Get("/:id/show", lobbyHandler.Show)
	lobbies.Post("/:lobbyId/join", lobbyHandler.JoinLobby)
	lobbies.Post("/:lobbyId/leave", lobbyHandler.LeaveLobby)
	lobbies.Post("/:lobbyId/invite", lobbyHandler.InviteUser)
	lobbies.Post("/invitation/accept", lobbyHandler.AcceptInvitation)
	lobbies.Post("/:lobbyId/ready", lobbyHandler.ReadyUp)
	api.Post("/lobbies/:id/invitations/accept", lobbyHandler.AcceptInvitation)

	// // Game Routes
	// games := api.Group("/games", middleware.AuthMiddleware(s.db))
	// games.Get("/:gameId", gameHandler.Show)
	// games.Post("/create", gameHandler.CreateGame)
	// games.Post("/:gameId/start", gameHandler.StartGame)
	// games.Post("/:gameId/deal", gameHandler.DealCards)
	// games.Post("/:gameId/play-card", gameHandler.PlayCard)
	// games.Get("/:gameId/deck", gameHandler.GetDeckInfo)

	profiles := api.Group("/profile", middleware.AuthMiddleware(s.db))
	profiles.Get("/:id/show", profileHandler.Show)
	profiles.Put("/:id/update", profileHandler.Update)
	profiles.Put("/:id/password", profileHandler.UpdatePassword)
	profiles.Delete("/:id/delete", profileHandler.Destroy)

	api.Get("/users/search", userHandler.SearchUsers)

	api.Get("/notifications", notificationHandler.GetNotifications)
	api.Put("/notifications/:id/read", notificationHandler.MarkAsRead)
	api.Put("/notifications/read-all", notificationHandler.MarkAllAsRead)
}
