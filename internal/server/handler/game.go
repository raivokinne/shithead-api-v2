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
	"github.com/google/uuid"
	"gorm.io/gorm"
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

		switch message.Type {
		case "game_action":
			h.handleGameAction(message)
		case "lobby_ready":
			payload, ok := message.Payload.(map[string]interface{})
			if !ok {
				log.Printf("Invalid payload format for lobby_ready: %v", message.Payload)
				break
			}

			lobbyID, ok := payload["lobbyId"].(string)

			if !ok || lobbyID == "" {
				log.Printf("Invalid or missing lobbyId in payload: %v", payload)
				break
			}

			userId := session.UserID

			tx := h.db.DB().Begin()

			var player models.Player
			if err := tx.Where("lobby_id = ? AND user_id = ?", lobbyID, userId).First(&player).Error; err != nil {
				tx.Rollback()
				log.Printf("Player not found in lobby: %v", payload)
				break
			}

			if player.IsReady {
				log.Print("Aready ready")
				h.hub.broadcast <- GameMessage{
					Type: "lobby_ready",
					Payload: fiber.Map{
						"message":  "Already ready",
						"is_ready": "true",
					},
				}
				break
			}

			if err := tx.Model(&player).Update("is_ready", "true").Error; err != nil {
				tx.Rollback()
				log.Print("Error updating player status")
				break
			}

			if err := tx.Commit().Error; err != nil {
				tx.Rollback()
				log.Print("Error committing transaction")
				break
			}

			h.hub.broadcast <- GameMessage{
				Type: "lobby_ready",
				Payload: fiber.Map{
					"message":  "Succesfully ready up",
					"is_ready": "true",
					"player":   player,
				},
			}
		case "play_card":
			payload, ok := message.Payload.(map[string]interface{})
			if !ok {
				log.Printf("Invalid payload format for play_card: %v", message.Payload)
				break
			}

			cardID, ok := payload["cardId"].(string)
			gameID, ok2 := payload["gameId"].(string)

			if !ok || !ok2  {
				log.Printf("Missing required fields in payload: %v", payload)
				break
			}

			tx := h.db.DB().Begin()

			parsedCardID, err := uuid.Parse(cardID)
			if err != nil {
				tx.Rollback()
				log.Printf("Invalid card ID: %v", err)
				break
			}

			parsedGameID, err := uuid.Parse(gameID)
			if err != nil {
				tx.Rollback()
				log.Printf("Invalid game ID: %v", err)
				break
			}

			var card models.Card
			if err := tx.Where("id = ?", parsedCardID).First(&card).Error; err != nil {
				tx.Rollback()
				log.Printf("Card not found: %v", err)
				break
			}

			updates := map[string]interface{}{
				"location_type": "play_pile",
				"player_id":     nil,
			}

			if err := tx.Model(&card).Updates(updates).Error; err != nil {
				tx.Rollback()
				log.Printf("Error updating card location: %v", err)
				break
			}

			if err := h.moveToNextPlayer(tx, parsedGameID); err != nil {
				tx.Rollback()
				log.Printf("Error moving to next player: %v", err)
				break
			}

			if err := tx.Commit().Error; err != nil {
				tx.Rollback()
				log.Printf("Error committing transaction: %v", err)
				break
			}

			h.hub.broadcast <- GameMessage{
				Type: "game_update",
				Payload: fiber.Map{
					"card_played": card,
					"game_id":     parsedGameID.String(),
				},
			}

		case "draw_card":
			payload, ok := message.Payload.(map[string]interface{})
			if !ok {
				log.Printf("Invalid payload format for draw_card: %v", message.Payload)
				break
			}

			playerID, ok := payload["playerId"].(string)
			if !ok {
				log.Printf("Missing playerID in payload: %v", payload)
				break
			}

			tx := h.db.DB().Begin()

			var card models.Card
			if err := tx.Where("location_type = ? AND player_id IS NULL", "deck").
				Order("random()").First(&card).Error; err != nil {
				tx.Rollback()
				log.Printf("No cards left in deck: %v", err)
				break
			}

			if err := tx.Model(&card).Updates(map[string]interface{}{
				"status":        "hand",
				"location_type": "hand",
				"player_id":     playerID,
			}).Error; err != nil {
				tx.Rollback()
				log.Printf("Error updating drawn card: %v", err)
				break
			}

			if err := tx.Commit().Error; err != nil {
				tx.Rollback()
				log.Printf("Error committing transaction: %v", err)
				break
			}

			h.hub.broadcast <- GameMessage{
				Type: "game_update",
				Payload: fiber.Map{
					"card_drawn": card,
					"player_id":  playerID,
				},
			}
		case "start_game":
			payload, ok := message.Payload.(map[string]interface{})
			if !ok {
				log.Printf("Invalid payload format for start_game: %v", message.Payload)
				break
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

func isValidPlay(card, topCard models.Card) bool {
	if topCard.ID == uuid.Nil {
		return true
	}

	if card.Value == "6" || card.Value == "10" {
		return true
	}

	return card.Value == topCard.Value
}

func (h *GameHandler) moveToNextPlayer(tx *gorm.DB, gameID uuid.UUID) error {
    var game models.Game
    if err := tx.Preload("Lobby").Preload("Lobby.Players").Where("id = ?", gameID).First(&game).Error; err != nil {
        return err
    }

    if len(game.Lobby.Players) == 0 {
        return fmt.Errorf("no players in the game lobby")
    }

    currentPlayerIndex := -1
    for i, player := range game.Lobby.Players {
        if player.ID == game.CurrentTurnPlayerID {
            currentPlayerIndex = i
            break
        }
    }

    if currentPlayerIndex == -1 {
        return fmt.Errorf("current player not found")
    }

    nextPlayerIndex := (currentPlayerIndex + 1) % len(game.Lobby.Players)

    game.CurrentTurnPlayerID = game.Lobby.Players[nextPlayerIndex].ID

    log.Printf("Next player index: %d, Player ID: %s", nextPlayerIndex, game.CurrentTurnPlayerID)

    return tx.Save(&game).Error
}

