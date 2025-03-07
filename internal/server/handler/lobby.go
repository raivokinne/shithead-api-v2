package handler

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"api/internal/database"
	"api/internal/database/models"
)

type LobbyHandler struct {
	db database.Service
}

type CreateLobbyRequest struct {
	Name             string          `json:"name" validate:"required"`
	Type             string          `json:"type" validate:"required,oneof=public private tournament"`
	Status           string          `json:"status" validate:"omitempty,oneof=waiting in_progress completed"`
	MaxPlayers       int             `json:"max_players" validate:"required,min=2,max=4"`
	GameMode         string          `json:"game_mode" validate:"omitempty,oneof=casual ranked tournament"`
	PrivacyLevel     string          `json:"privacy_level" validate:"omitempty,oneof=open invite_only password_protected"`
	Password         string          `json:"password" validate:"omitempty,min=6"`
	SpectatorAllowed bool            `json:"spectator_allowed"`
	GameSettings     json.RawMessage `json:"game_settings"`
}

type JoinLobbyRequest struct {
	InviteCode string `json:"invite_code,omitempty"`
	Password   string `json:"password,omitempty"`
}

type InviteUserRequest struct {
	InvitedUserID uuid.UUID `json:"invited_user_id" validate:"required"`
}

type AcceptInvitationRequest struct {
	LobbyID uuid.UUID `json:"lobby_id" validate:"required"`
}

func NewLobbyHandler(db database.Service) *LobbyHandler {
	return &LobbyHandler{
		db: db,
	}
}

func generateInviteCode() string {
	bytes := make([]byte, 2)
	rand.Read(bytes)
	return hex.EncodeToString(bytes)
}

func (h *LobbyHandler) Index(c *fiber.Ctx) error {
	sessionID := c.Cookies("session_id")
	if sessionID == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "Session ID not provided",
		})
	}

	var session models.Session
	if err := h.db.DB().Where("id = ?", sessionID).First(&session).Error; err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "Invalid session",
		})
	}

	var currentUser models.User
	if err := h.db.DB().First(&currentUser, session.UserID).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Error fetching user",
		})
	}

	var lobbies []models.Lobby
	if err := h.db.DB().
		Preload("Owner").
		Preload("Players").
		Preload("LobbyInvitations").
		Preload("Games").
		Preload("LobbyQueues").
		Find(&lobbies).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Error fetching lobbies",
		})
	}

	formattedLobbies := make([]fiber.Map, len(lobbies))
	for i, lobby := range lobbies {
		formattedLobbies[i] = h.formatLobbyResponse(lobby, currentUser)
	}

	return c.JSON(formattedLobbies)
}

func (h *LobbyHandler) Store(c *fiber.Ctx) error {
	var req CreateLobbyRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	sessionID := c.Cookies("session_id")
	if sessionID == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "Session ID not provided",
		})
	}

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

	// Check existing lobby and player
	var existingLobby models.Lobby
	err := h.db.DB().Where("owner_id = ?", user.ID).First(&existingLobby).Error
	if err == nil {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"error": "You already have an active lobby",
		})
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Error checking user's lobbies",
		})
	}

	var existingPlayer models.Player
	err = h.db.DB().Where("user_id = ?", user.ID).First(&existingPlayer).Error
	if err == nil {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"error": "You are already in another lobby",
		})
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Error checking user's player status",
		})
	}

	var passwordHash *string
	if req.Password != "" {
		hashedPass, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "Error hashing password",
			})
		}
		hashStr := string(hashedPass)
		passwordHash = &hashStr
	}
	tx := h.db.DB().Begin()

	lobby := models.Lobby{
		ID:               uuid.New(),
		Name:             req.Name,
		Type:             req.Type,
		OwnerID:          user.ID,
		Status:           req.Status,
		MaxPlayers:       req.MaxPlayers,
		GameMode:         req.GameMode,
		PrivacyLevel:     req.PrivacyLevel,
		PasswordHash:     passwordHash,
		SpectatorAllowed: req.SpectatorAllowed,
		GameSettings:     req.GameSettings,
		CurrentPlayers:   1,
	}

	if err := tx.Create(&lobby).Error; err != nil {
		tx.Rollback()
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Error creating lobby",
		})
	}

	gameID := uuid.New()
	game := models.Game{
		ID:                  gameID,
		LobbyID:             lobby.ID,
		Status:              "waiting",
		OwnerID:             user.ID,
		CurrentTurnPlayerID: uuid.Nil,
		RoundNumber:         1,
		Winner:              "none",
	}

	if err := tx.Create(&game).Error; err != nil {
		tx.Rollback()
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Error creating game",
		})
	}

	max := big.NewInt(4)
	randomIndex, err := rand.Int(rand.Reader, max)
	if err != nil {
		tx.Rollback()
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Error generating random role",
		})
	}

	player := models.Player{
		ID:      uuid.New(),
		LobbyID: lobby.ID,
		GameID:  gameID,
		UserID:  user.ID,
		Role:    fmt.Sprintf("player%d", randomIndex.Int64()+1),
		IsReady: false,
		Score:   0,
	}

	if err := tx.Create(&player).Error; err != nil {
		tx.Rollback()
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Error creating player",
		})
	}

	if err := tx.Model(&game).Update("current_turn_player_id", player.ID).Error; err != nil {
		tx.Rollback()
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Error updating game with player ID",
		})
	}

	if err := tx.Commit().Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Error committing transaction",
		})
	}

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"lobby": lobby,
	})
}

func (h *LobbyHandler) Show(c *fiber.Ctx) error {
	lobbyID := c.Params("id")

	sessionID := c.Cookies("session_id")
	if sessionID == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "Session ID not provided",
		})
	}

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

	var lobby models.Lobby
	if err := h.db.DB().Preload("Owner").Preload("Players.User").Preload("Games").
		Preload("LobbyInvitations").Where("id = ?", lobbyID).First(&lobby).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Lobby not found",
		})
	}

	response := h.formatLobbyResponse(lobby, user)
	return c.JSON(response)
}

func (h *LobbyHandler) JoinLobby(c *fiber.Ctx) error {
	lobbyID, err := uuid.Parse(c.Params("lobbyId"))

	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "Wrong lobby id",
		})
	}

	sessionID := c.Cookies("session_id")
	if sessionID == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "Session ID not provided",
		})
	}

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

	var req JoinLobbyRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	tx := h.db.DB().Begin()

	var lobby models.Lobby
	if err := tx.Preload("Players").Preload("LobbyInvitations").
		First(&lobby, lobbyID).Error; err != nil {
		tx.Rollback()
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Lobby not found",
		})
	}

	if lobby.Status != "waiting" {
		tx.Rollback()
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Lobby not accepting players",
		})
	}

	var existingPlayer models.Player
	if err := tx.Where("lobby_id = ? AND user_id = ?", lobbyID, user.ID).First(&existingPlayer).Error; err == nil {
		if err := tx.Commit().Error; err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "Error committing transaction",
			})
		}
		return c.JSON(fiber.Map{
			"message":  "Successfully joined lobby",
			"lobby_id": lobby.ID,
		})
	}

	var currentUser models.User
	if err := tx.First(&currentUser, user.ID).Error; err != nil {
		tx.Rollback()
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Error fetching user",
		})
	}

	var currentPlayer *models.Player
	for _, player := range lobby.Players {
		if player.UserID == currentUser.ID {
			currentPlayer = &player
			break
		}
	}

	if existingPlayer.ID != currentPlayer.ID {
		switch lobby.PrivacyLevel {
		case "password_protected":
			if err := h.handlePasswordProtectedJoin(&lobby, req.Password); err != nil {
				tx.Rollback()
				return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
					"error": "Error committing transaction",
				})
			}
		}
	}

	if lobby.CurrentPlayers >= lobby.MaxPlayers {
		return h.handleQueueJoin(tx, c, &lobby, user.ID)
	}

	if err := h.addPlayerToLobby(tx, &lobby, user.ID); err != nil {
		tx.Rollback()
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Error committing transaction",
		})
	}

	if err := tx.Commit().Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Error committing transaction",
		})
	}

	return c.JSON(fiber.Map{
		"message":  "Successfully joined lobby",
		"lobby_id": lobby.ID,
	})
}

func (h *LobbyHandler) LeaveLobby(c *fiber.Ctx) error {
	lobbyID := c.Params("lobbyId")
	userID := c.Locals("user_id").(uuid.UUID)

	tx := h.db.DB().Begin()

	var lobby models.Lobby
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", lobbyID).First(&lobby).Error; err != nil {
		tx.Rollback()
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Lobby not found",
		})
	}

	if lobby.OwnerID == userID {
		if err := h.deleteLobbyAndRelatedRecords(tx, lobbyID); err != nil {
			tx.Rollback()
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "Error deleting lobby and related records",
			})
		}

		if err := tx.Commit().Error; err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "Error committing transaction",
			})
		}

		return c.JSON(fiber.Map{
			"message": "Successfully deleted lobby",
		})
	}

	var player models.Player
	if err := tx.Where("lobby_id = ? AND user_id = ?", lobbyID, userID).First(&player).Error; err != nil {
		tx.Rollback()
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Not in lobby",
		})
	}

	if err := tx.Delete(&player).Error; err != nil {
		tx.Rollback()
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Error removing player",
		})
	}

	if err := tx.Model(&lobby).Update("current_players", gorm.Expr("current_players - ?", 1)).Error; err != nil {
		tx.Rollback()
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Error updating player count",
		})
	}

	if err := tx.Where("lobby_id = ? AND user_id = ?", lobbyID, userID).Delete(&models.LobbyQueue{}).Error; err != nil {
		tx.Rollback()
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Error removing from queue",
		})
	}

	if err := tx.Commit().Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Error committing transaction",
		})
	}

	return c.JSON(fiber.Map{
		"message": "Successfully left lobby",
	})
}

func (h *LobbyHandler) deleteLobbyAndRelatedRecords(tx *gorm.DB, lobbyID string) error {
	if err := tx.Where("lobby_id = ?", lobbyID).Delete(&models.LobbyInvitation{}).Error; err != nil {
		return err
	}

	if err := tx.Where("lobby_id = ?", lobbyID).Delete(&models.Player{}).Error; err != nil {
		return err
	}

	if err := tx.Where("lobby_id = ?", lobbyID).Delete(&models.LobbyQueue{}).Error; err != nil {
		return err
	}

	if err := tx.Where("lobby_id = ?", lobbyID).Delete(&models.Game{}).Error; err != nil {
		return err
	}

	if err := tx.Where("data->>'lobby_id' = ?", lobbyID).Delete(&models.Notification{}).Error; err != nil {
		return err
	}

	if err := tx.Where("id = ?", lobbyID).Delete(&models.Lobby{}).Error; err != nil {
		return err
	}

	return nil
}

func (h *LobbyHandler) InviteUser(c *fiber.Ctx) error {
	lobbyID := c.Params("lobbyId")

	sessionID := c.Cookies("session_id")
	var session models.Session
	if err := h.db.DB().Where("id = ?", sessionID).First(&session).Error; err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "Invalid session",
		})
	}

	var currentUser models.User
	if err := h.db.DB().First(&currentUser, session.UserID).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Error fetching user",
		})
	}

	var req InviteUserRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	if req.InvitedUserID == currentUser.ID {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Cannot invite yourself",
		})
	}

	var lobby models.Lobby
	if err := h.db.DB().Where("id = ?", lobbyID).Preload("Owner").First(&lobby).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Lobby not found",
		})
	}

	if lobby.OwnerID != currentUser.ID {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"error": "Only the lobby owner can send invitations",
		})
	}

	if lobby.CurrentPlayers >= lobby.MaxPlayers {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Lobby is full",
		})
	}

	var existingInvitation models.LobbyInvitation
	existingErr := h.db.DB().Where("lobby_id = ? AND invited_user_id = ? AND status = ?",
		lobbyID, req.InvitedUserID, "pending").First(&existingInvitation).Error
	if existingErr == nil {
		return c.Status(fiber.StatusConflict).JSON(fiber.Map{
			"error": "Invitation already exists for this user",
		})
	}

	now := time.Now().UTC()
	invitation := models.LobbyInvitation{
		ID:            uuid.New(),
		LobbyID:       lobby.ID,
		InviterID:     currentUser.ID,
		InvitedUserID: req.InvitedUserID,
		Status:        "pending",
		ExpiresAt:     now.Add(30 * time.Minute),
		CreatedAt:     &now,
		UpdatedAt:     &now,
	}

	tx := h.db.DB().Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	if err := tx.Create(&invitation).Error; err != nil {
		tx.Rollback()
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to create invitation",
		})
	}

	messageType := "lobby_invitation"
	notification := models.Notification{
		ID:     uuid.New(),
		Type:   &messageType,
		UserID: req.InvitedUserID,
		Data: json.RawMessage(
			fmt.Sprintf(
				`{"lobby_id": "%s", "expires_at": "%s", "lobby_name": "%s", "message": "You have been invited to a lobby"}`,
				lobby.ID,
				invitation.ExpiresAt,
				lobby.Name,
			),
		),
		CreatedAt: now,
		UpdatedAt: now,
	}

	if err := tx.Create(&notification).Error; err != nil {
		tx.Rollback()
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to create notification",
		})
	}

	if err := tx.Commit().Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to commit transaction",
		})
	}

	return c.JSON(fiber.Map{
		"message": "Invitation sent successfully",
		"invitation": fiber.Map{
			"expires_at": invitation.ExpiresAt,
		},
	})
}

func (h *LobbyHandler) AcceptInvitation(c *fiber.Ctx) error {
	var req AcceptInvitationRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	sessionID := c.Cookies("session_id")
	var session models.Session
	if err := h.db.DB().Where("id = ?", sessionID).First(&session).Error; err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "Invalid session",
		})
	}

	userID := session.UserID
	tx := h.db.DB().Begin()

	fmt.Printf("Looking for invitation with lobby_id: %s and user_id: %s\n", req.LobbyID, userID)

	var invitation models.LobbyInvitation
	if err := tx.Debug().Where("lobby_id = ? AND invited_user_id = ?",
		req.LobbyID, userID).First(&invitation).Error; err != nil {
		fmt.Printf("Error finding invitation: %v\n", err)
		tx.Rollback()
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Invalid invitation",
		})
	}

	if invitation.ExpiresAt.Before(time.Now()) {
		tx.Rollback()
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invitation has expired",
		})
	}

	if invitation.Status != "pending" {
		tx.Rollback()
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invitation has already been processed",
		})
	}

	var lobby *models.Lobby
	if err := tx.First(&lobby, invitation.LobbyID).Error; err != nil {
		tx.Rollback()
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Lobby not found",
		})
	}

	if lobby.CurrentPlayers >= lobby.MaxPlayers {
		tx.Rollback()
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Lobby is full",
		})
	}

	if err := tx.Model(&invitation).Updates(map[string]interface{}{
		"status":     "accepted",
		"updated_at": time.Now(),
	}).Error; err != nil {
		tx.Rollback()
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Error updating invitation",
		})
	}

	if err := h.addPlayerToLobby(tx, lobby, userID); err != nil {
		tx.Rollback()
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Error adding user to lobby",
		})
	}

	if err := tx.Model(&lobby).Updates(map[string]interface{}{
		"current_players": lobby.CurrentPlayers + 1,
		"updated_at":      time.Now(),
	}).Error; err != nil {
		tx.Rollback()
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Error updating lobby player count",
		})
	}

	if err := tx.Commit().Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Error committing transaction",
		})
	}

	return c.JSON(fiber.Map{
		"success": true,
		"message": "Successfully joined lobby",
		"lobby":   lobby,
	})
}

func (h *LobbyHandler) handlePasswordProtectedJoin(lobby *models.Lobby, password string) error {
	if password == "" || !checkPasswordHash(password, *lobby.PasswordHash) {
		return &fiber.Error{
			Code:    fiber.StatusUnauthorized,
			Message: "Invalid password",
		}
	}
	return nil
}

func (h *LobbyHandler) handleQueueJoin(tx *gorm.DB, c *fiber.Ctx, lobby *models.Lobby, userID uuid.UUID) error {
	var existingQueue models.LobbyQueue
	if err := tx.Where("lobby_id = ? AND user_id = ?", lobby.ID, userID).First(&existingQueue).Error; err == nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Already in queue",
		})
	}

	queuePosition := int(1)
	var lastQueue models.LobbyQueue
	if err := tx.Where("lobby_id = ?", lobby.ID).Order("queue_position desc").First(&lastQueue).Error; err == nil {
		queuePosition = *lastQueue.Position + int(1)
	}

	queue := models.LobbyQueue{
		LobbyID:   lobby.ID,
		UserID:    userID,
		QueueType: "player",
		Position:  &queuePosition,
	}

	if err := tx.Create(&queue).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Error joining queue",
		})
	}

	if err := tx.Commit().Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Error committing transaction",
		})
	}

	return c.Status(fiber.StatusAccepted).JSON(fiber.Map{
		"message":        "Added to queue",
		"queue_position": queuePosition,
	})
}

func (h *LobbyHandler) handleExistingPlayer(tx *gorm.DB, c *fiber.Ctx, lobby *models.Lobby, userID uuid.UUID) error {
	if err := h.addPlayerToLobby(tx, lobby, userID); err != nil {
		return err
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"message": "Successfully joined already existing lobby",
	})
}

func (h *LobbyHandler) addPlayerToLobby(tx *gorm.DB, lobby *models.Lobby, userID uuid.UUID) error {
	var game models.Game
	err := tx.Where("lobby_id = ? AND status = ?", lobby.ID, "waiting").First(&game).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		game = models.Game{
			LobbyID:     lobby.ID,
			RoundNumber: 1,
			Status:      "waiting",
			Winner:      "none",
		}
		if err := tx.Create(&game).Error; err != nil {
			return err
		}
	} else if err != nil {
		return err
	}

	var existingPlayer models.Player
	if err := tx.Where("lobby_id = ? AND user_id = ?", lobby.ID, userID).First(&existingPlayer).Error; err == nil {
		return nil
	}

	playerNumber := lobby.CurrentPlayers
	player := models.Player{
		ID:      uuid.New(),
		LobbyID: lobby.ID,
		GameID:  game.ID,
		UserID:  userID,
		Role:    fmt.Sprintf("player%d", playerNumber),
		Score:   0,
	}

	if err := tx.Model(&lobby).Update("current_players", gorm.Expr("current_players + ?", 1)).Error; err != nil {
		return err
	}

	if err := tx.Create(&player).Error; err != nil {
		return err
	}

	if err := tx.Save(lobby).Error; err != nil {
		return err
	}
	return nil
}

func (h *LobbyHandler) formatLobbyResponse(lobby models.Lobby, currentUser models.User) fiber.Map {
	var currentGame *models.Game
	if len(lobby.Games) > 0 {
		currentGame = &lobby.Games[0]
	}

	var currentPlayer *models.Player
	for _, player := range lobby.Players {
		if player.UserID == currentUser.ID {
			currentPlayer = &player
			break
		}
	}

	return fiber.Map{
		"id":   lobby.ID,
		"name": lobby.Name,
		"owner": fiber.Map{
			"id":   lobby.Owner.ID,
			"name": lobby.Owner.Name,
		},
		"max_players":       lobby.MaxPlayers,
		"current_user":      currentUser,
		"is_player":         currentPlayer != nil,
		"player_role":       getPlayerRole(currentPlayer),
		"current_players":   lobby.CurrentPlayers,
		"status":            lobby.Status,
		"type":              lobby.Type,
		"game_mode":         lobby.GameMode,
		"participants":      h.formatParticipants(lobby.Players),
		"current_game":      h.formatGame(currentGame),
		"spectator_allowed": lobby.SpectatorAllowed,
		"game_settings":     lobby.GameSettings,
		"queue":             h.formatQueue(lobby.LobbyQueues),
		"created_at":        lobby.CreatedAt,
		"updated_at":        lobby.UpdatedAt,
		"privacy_level":     lobby.PrivacyLevel,
	}
}

func (h *LobbyHandler) formatParticipants(players []models.Player) []fiber.Map {
	result := make([]fiber.Map, len(players))
	for i, player := range players {
		var user models.User
		if err := h.db.DB().First(&user, player.UserID).Error; err != nil {
			continue
		}
		result[i] = fiber.Map{
			"id":       user.ID,
			"name":     user.Name,
			"role":     player.Role,
			"score":    player.Score,
			"is_ready": player.IsReady,
		}
	}
	return result
}

func (h *LobbyHandler) formatGame(game *models.Game) *fiber.Map {
	if game == nil {
		return nil
	}
	return &fiber.Map{
		"id":           game.ID,
		"status":       game.Status,
		"round_number": game.RoundNumber,
	}
}

func (h *LobbyHandler) formatQueue(queue []models.LobbyQueue) []fiber.Map {
	result := make([]fiber.Map, len(queue))
	for i, item := range queue {
		result[i] = fiber.Map{
			"id":         item.User.ID,
			"name":       item.User.Name,
			"queue_type": item.QueueType,
		}
	}
	return result
}

func checkPasswordHash(password string, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}

func getPlayerRole(player *models.Player) string {
	if player == nil {
		return ""
	}
	return player.Role
}
