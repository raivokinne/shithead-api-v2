package handler

import (
	"api/internal/database"
	"api/internal/database/models"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Card struct {
	Code  string `json:"code"`
	Image string `json:"image"`
	Value string `json:"value"`
	Suit  string `json:"suit"`
}

type Deck struct {
	Success   bool   `json:"success"`
	DeckID    string `json:"deck_id"`
	Cards     []Card `json:"cards"`
	Remaining int    `json:"remaining"`
}

type GameCard struct {
	ID           uuid.UUID  `json:"id"`
	Code         string     `json:"code"`
	Value        string     `json:"value"`
	Suit         string     `json:"suit"`
	ImageURL     string     `json:"image_url,omitempty"`
	Status       string     `json:"status"`
	LocationType string     `json:"location_type"`
	PlayerID     *uuid.UUID `json:"player_id,omitempty"`
}

type GameState struct {
	ID              uuid.UUID       `json:"id"`
	Status          string          `json:"status"`
	CurrentPlayerID uuid.UUID       `json:"current_player_id,omitempty"`
	RoundNumber     int             `json:"round_number"`
	Players         []PlayerSummary `json:"players"`
	LobbyInfo       LobbyInfo       `json:"lobby"`
	Game            models.Game     `json:"game"`
}

type PlayerSummary struct {
	ID        uuid.UUID `json:"id"`
	Name      string    `json:"name"`
	Email     string    `json:"email"`
	Avatar    *string   `json:"avatar,omitempty"`
	CardCount int64     `json:"card_count"`
	IsCurrent bool      `json:"is_current"`
}

type LobbyInfo struct {
	ID             uuid.UUID `json:"id"`
	Name           string    `json:"name"`
	OwnerName      string    `json:"owner_name"`
	Type           string    `json:"type"`
	MaxPlayers     int       `json:"max_players"`
	CurrentPlayers int       `json:"current_players"`
	GameMode       string    `json:"game_mode"`
}

type CardHandler struct {
	db database.Service
}

func NewCardHandler(db database.Service) *CardHandler {
	return &CardHandler{db: db}
}

func (h *CardHandler) GetGameCards(c *fiber.Ctx) error {
	sessionId := c.Cookies("session_id")
	var session models.Session
	if err := h.db.DB().
		Where("id = ?", sessionId).
		First(&session).Error; err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "Invalid session",
		})
	}

	gameId := c.Params("gameId")
	if gameId == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Game ID is required",
		})
	}

	gameUUID, err := uuid.Parse(gameId)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid game ID format",
		})
	}

	var player models.Player
	if err := h.db.DB().
		Where("user_id = ? AND game_id = ?", session.UserID, gameUUID).
		First(&player).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Player not found in game",
		})
	}

	var game models.Game
	if err := h.db.DB().
		Preload("Lobby").
		Preload("Lobby.Owner").
		Where("id = ?", gameUUID).
		First(&game).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Game not found",
		})
	}

	players, err := h.getPlayerSummaries(gameId, game.CurrentTurnPlayerID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": fmt.Sprintf("Failed to get player information: %v", err),
		})
	}

	cards, err := h.getOrCreateGameCards(gameId)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": fmt.Sprintf("Failed to get or create game cards: %v", err),
		})
	}

	if len(cards) == 0 {
		if err := h.db.DB().
			Where("game_id = ?", gameUUID).
			Find(&cards).Error; err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "Failed to fetch existing cards",
			})
		}
	}

	gameState := GameState{
		ID:              game.ID,
		Status:          game.Status,
		CurrentPlayerID: game.CurrentTurnPlayerID,
		RoundNumber:     game.RoundNumber,
		Players:         players,
		Game:            game,
		LobbyInfo: LobbyInfo{
			ID:             game.Lobby.ID,
			Name:           game.Lobby.Name,
			OwnerName:      game.Lobby.Owner.Name,
			Type:           game.Lobby.Type,
			MaxPlayers:     game.Lobby.MaxPlayers,
			CurrentPlayers: game.Lobby.CurrentPlayers,
			GameMode:       game.Lobby.GameMode,
		},
	}

	gameCards := make([]GameCard, len(cards))
	for i, card := range cards {
		gameCards[i] = GameCard{
			ID:           card.ID,
			Code:         card.Code,
			Value:        card.Value,
			Suit:         card.Suit,
			ImageURL:     *card.ImageURL,
			Status:       card.Status,
			LocationType: card.LocationType,
			PlayerID:     card.PlayerID,
		}
	}

	return c.JSON(fiber.Map{
		"cards":      gameCards,
		"game_state": gameState,
	})
}

func (h *CardHandler) getOrCreateGameCards(gameId string) ([]models.Card, error) {
	var cards []models.Card
	var existingDeck models.Deck

	gameUUID, err := uuid.Parse(gameId)
	if err != nil {
		return nil, fmt.Errorf("invalid game ID format: %v", err)
	}

	tx := h.db.DB().Begin()
	if tx.Error != nil {
		return nil, fmt.Errorf("error starting transaction: %v", tx.Error)
	}

	if err := tx.Exec("SET TRANSACTION ISOLATION LEVEL SERIALIZABLE").Error; err != nil {
		tx.Rollback()
		return nil, fmt.Errorf("error setting isolation level: %v", err)
	}

	deckErr := tx.Where("game_id = ?", gameUUID).First(&existingDeck).Error
	if !errors.Is(deckErr, gorm.ErrRecordNotFound) {
		if err := tx.Where("game_id = ?", gameUUID).Find(&cards).Error; err != nil {
			tx.Rollback()
			return nil, fmt.Errorf("error fetching existing cards: %v", err)
		}
		tx.Commit()
		return cards, nil
	}
	if errors.Is(deckErr, gorm.ErrRecordNotFound) {
		log.Printf("No deck found, creating a new deck for game %s", gameId)
		tx := h.db.DB().Begin()
		if tx.Error != nil {
			return nil, fmt.Errorf("error starting transaction: %v", tx.Error)
		}

		defer func() {
			if r := recover(); r != nil {
				tx.Rollback()
			}
		}()

		deck := models.Deck{
			ID:             uuid.New(),
			GameID:         gameUUID,
			DeckType:       "standard",
			TotalCards:     52,
			RemainingCards: 52,
			DeckConfiguration: json.RawMessage(`{
            "includeJokers": false,
            "specialCards": {
                "6": "reset_deck",
                "10": "clear_deck_extra_move"
            }
        }`),
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}

		log.Printf("Creating new deck for game %s", gameId)
		if err := tx.Create(&deck).Error; err != nil {
			tx.Rollback()
			return nil, fmt.Errorf("error creating deck: %v", err)
		}

		var players []models.Player
		if err := tx.Where("game_id = ?", gameUUID).Find(&players).Error; err != nil {
			tx.Rollback()
			return nil, fmt.Errorf("error fetching players: %v", err)
		}

		if len(players) == 0 {
			tx.Rollback()
			return nil, fmt.Errorf("no players found for game %s", gameId)
		}

		apiCards, err := FetchAllCards()
		if err != nil {
			tx.Rollback()
			return nil, fmt.Errorf("error fetching cards from API: %v", err)
		}

		if len(apiCards) != 52 {
			tx.Rollback()
			return nil, fmt.Errorf("expected 52 cards from API, got %d", len(apiCards))
		}

		cards = make([]models.Card, 0, 52)
		cardIndex := 0

		for _, player := range players {
			for _, status := range []string{"hidden", "faceup", "hand"} {
				for i := 0; i < 3; i++ {
					if cardIndex >= len(apiCards) {
						tx.Rollback()
						return nil, fmt.Errorf("not enough cards for distribution at index %d", cardIndex)
					}

					card := models.Card{
						ID:            uuid.New(),
						DeckID:        deck.ID,
						GameID:        gameUUID,
						Code:          apiCards[cardIndex].Code,
						Value:         apiCards[cardIndex].Value,
						Suit:          apiCards[cardIndex].Suit,
						ImageURL:      &apiCards[cardIndex].Image,
						Status:        status,
						LocationType:  "player",
						PlayerID:      &player.ID,
						IsSpecialCard: isSpecialCard(apiCards[cardIndex].Value),
						SpecialAction: getSpecialAction(apiCards[cardIndex].Value),
						CreatedAt:     time.Now(),
						UpdatedAt:     time.Now(),
					}
					cards = append(cards, card)
					cardIndex++
				}
			}
		}

		for i := cardIndex; i < len(apiCards); i++ {
			card := models.Card{
				ID:            uuid.New(),
				DeckID:        deck.ID,
				GameID:        gameUUID,
				Code:          apiCards[i].Code,
				Value:         apiCards[i].Value,
				Suit:          apiCards[i].Suit,
				ImageURL:      &apiCards[i].Image,
				Status:        "in_deck",
				LocationType:  "deck",
				IsSpecialCard: isSpecialCard(apiCards[i].Value),
				SpecialAction: getSpecialAction(apiCards[i].Value),
				CreatedAt:     time.Now(),
				UpdatedAt:     time.Now(),
			}
			cards = append(cards, card)
		}

		deck.RemainingCards = len(apiCards) - cardIndex
		if err := tx.Save(&deck).Error; err != nil {
			tx.Rollback()
			return nil, fmt.Errorf("error updating deck remaining cards: %v", err)
		}

		log.Printf("Creating %d cards for game %s", len(cards), gameId)
		if err := tx.Create(&cards).Error; err != nil {
			tx.Rollback()
			return nil, fmt.Errorf("error creating cards: %v", err)
		}

		if err := tx.Commit().Error; err != nil {
			return nil, fmt.Errorf("error committing transaction: %v", err)
		}

		log.Printf("Successfully created deck and distributed %d cards for game %s", len(cards), gameId)
	}
	return cards, nil
}

func FetchAllCards() ([]Card, error) {
	client := &http.Client{
		Timeout: time.Second * 10,
	}

	resp, err := client.Get("https://deckofcardsapi.com/api/deck/new/shuffle/")
	if err != nil {
		return nil, fmt.Errorf("error creating new deck: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading response body: %v", err)
	}

	fmt.Printf("Deck creation response: %s\n", string(body))

	var deckResp struct {
		Success bool   `json:"success"`
		DeckID  string `json:"deck_id"`
	}
	if err := json.Unmarshal(body, &deckResp); err != nil {
		return nil, fmt.Errorf("error decoding deck response: %v", err)
	}

	if !deckResp.Success {
		return nil, fmt.Errorf("deck creation unsuccessful")
	}

	drawURL := fmt.Sprintf("https://deckofcardsapi.com/api/deck/%s/draw/?count=52", deckResp.DeckID)
	drawResp, err := client.Get(drawURL)
	if err != nil {
		return nil, fmt.Errorf("error drawing cards: %v", err)
	}
	defer drawResp.Body.Close()

	body, err = io.ReadAll(drawResp.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading draw response body: %v", err)
	}

	fmt.Printf("Draw cards response: %s\n", string(body))

	var deck Deck
	if err := json.Unmarshal(body, &deck); err != nil {
		return nil, fmt.Errorf("error decoding cards response: %v", err)
	}

	if !deck.Success {
		return nil, fmt.Errorf("card draw unsuccessful")
	}

	if len(deck.Cards) != 52 {
		return nil, fmt.Errorf("expected 52 cards, got %d", len(deck.Cards))
	}

	return deck.Cards, nil
}

func isSpecialCard(value string) bool {
	specialValues := map[string]bool{
		"6":  true,
		"10": true,
	}
	return specialValues[value]
}

func getSpecialAction(value string) string {
	specialActions := map[string]string{
		"6":  "any",
		"10": "clear",
		"":   "none",
	}
	action, exists := specialActions[value]
	if !exists {
		return "none"
	}
	return action
}

func (h *CardHandler) getPlayerSummaries(gameId string, currentPlayerID uuid.UUID) ([]PlayerSummary, error) {
	var players []models.Player
	if err := h.db.DB().
		Preload("User").
		Where("game_id = ?", gameId).
		Find(&players).Error; err != nil {
		return nil, err
	}

	summaries := make([]PlayerSummary, len(players))
	for i, p := range players {
		var cardCount int64
		h.db.DB().Model(&models.Card{}).Where("player_id = ?", p.ID).Count(&cardCount)

		summaries[i] = PlayerSummary{
			ID:        p.ID,
			Name:      p.User.Name,
			Email:     p.User.Email,
			Avatar:    p.User.Avatar,
			CardCount: cardCount,
			IsCurrent: p.ID == currentPlayerID,
		}
	}

	return summaries, nil
}
