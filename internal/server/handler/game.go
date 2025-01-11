package handler

import (
	"api/internal/database"
	"api/internal/database/models"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/gofiber/contrib/websocket"
	"github.com/gofiber/fiber/v2"
)

type GameMessage struct {
	Type    string      `json:"type"`
	Payload interface{} `json:"payload"`
}

type Client struct {
	UserId string
	GameId string
}

type GameHub struct {
	clients    map[*websocket.Conn]Client
	register   chan *websocket.Conn
	unregister chan *websocket.Conn
	broadcast  chan GameMessage
}

func NewGameHub() *GameHub {
	return &GameHub{
		clients:    make(map[*websocket.Conn]Client),
		register:   make(chan *websocket.Conn),
		unregister: make(chan *websocket.Conn),
		broadcast:  make(chan GameMessage),
	}
}

func (h *GameHub) Run() {
	for {
		select {
		case conn := <-h.register:
			h.clients[conn] = Client{}

		case conn := <-h.unregister:
			if _, ok := h.clients[conn]; ok {
				delete(h.clients, conn)
				conn.Close()
			}

		case message := <-h.broadcast:
			messageBytes, err := json.Marshal(message)
			if err != nil {
				continue
			}

			for connection := range h.clients {
				if err := connection.WriteMessage(websocket.TextMessage, messageBytes); err != nil {
					h.unregister <- connection
					connection.WriteMessage(websocket.CloseMessage, []byte{})
					connection.Close()
				}
			}
		}
	}
}

type GameHandler struct {
	db   database.Service
	hub  *GameHub
	once sync.Once
}

func NewGameHandler(db database.Service) *GameHandler {
	return &GameHandler{
		db:  db,
		hub: NewGameHub(),
	}
}

func (h *GameHandler) Game(c *websocket.Conn) {
	h.once.Do(func() {
		go h.hub.Run()
	})

	h.hub.register <- c

	defer func() {
		h.hub.unregister <- c
	}()

	for {
		_, messageBytes, err := c.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("Error reading message: %v", err)
			}
			return
		}

		var message GameMessage
		if err := json.Unmarshal(messageBytes, &message); err != nil {
			log.Printf("Error parsing message: %v", err)
			continue
		}

		switch message.Type {
		case "game_action":
			h.handleGameAction(message)
		case "start_game":
			sessionId := c.Cookies("session_id")
			var session models.Session
			if err := h.db.DB().Where("id = ?", sessionId).First(&session).Error; err != nil {
				h.hub.broadcast <- GameMessage{
					Type: "game_error",
					Payload: fiber.Map{
						"error": "Invalid Session",
					},
				}
			}

			payload, ok := message.Payload.(map[string]interface{})
			if !ok {
				log.Printf("Invalid payload format for start_game: %v", message.Payload)
				continue
			}

			gameId, ok := payload["gameId"].(string)
			if !ok || gameId == "" {
				log.Printf("Invalid or missing gameId in payload: %v", payload)
				continue
			}

			var game models.Game
			if err := h.db.DB().Preload("Lobby.Players").
				Where("id = ?", gameId).
				First(&game).Error; err != nil {
				log.Printf("Game not found with ID: %s, error: %v", gameId, err)
				continue
			}

			if game.Status != "waiting" {
				log.Printf("Game with ID %s is not in waiting status. Current status: %s", gameId, game.Status)
				continue
			}

			game.Status = "in_progress"
			game.UpdatedAt = time.Now()
			if err := h.db.DB().Save(&game).Error; err != nil {
				log.Printf("Failed to update game status for ID %s: %v", gameId, err)
				continue
			}

			h.hub.broadcast <- GameMessage{
				Type: "game_started",
				Payload: fiber.Map{
					"game_id":  game.ID,
					"players":  game.Lobby.Players,
					"redirect": fmt.Sprintf("/games/%s", game.ID),
				},
			}
		default:
			log.Printf("Unknown message type: %s", message.Type)
		}
	}
}

func (h *GameHandler) handleGameAction(message GameMessage) {
	h.hub.broadcast <- GameMessage{
		Type:    "game_update",
		Payload: message.Payload,
	}
}
