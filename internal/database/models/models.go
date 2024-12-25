package models

import (
	"encoding/json"
	"github.com/google/uuid"
	"time"
)

type User struct {
	ID              uuid.UUID      `gorm:"primaryKey;column:id" json:"id"`
	Name            string         `gorm:"column:name;not null" json:"name"`
	Email           string         `gorm:"column:email;unique;not null" json:"email"`
	EmailVerifiedAt *time.Time     `gorm:"column:email_verified_at" json:"email_verified_at"`
	Password        string         `gorm:"column:password;not null" json:"password"`
	Avatar          *string        `gorm:"column:avatar" json:"avatar"`
	RememberToken   *string        `gorm:"column:remember_token;size:100" json:"remember_token"`
	CreatedAt       *time.Time     `gorm:"column:created_at" json:"created_at"`
	UpdatedAt       *time.Time     `gorm:"column:updated_at" json:"updated_at"`
	Lobbies         []Lobby        `gorm:"foreignKey:OwnerID" json:"lobbies"`
	Players         []Player       `gorm:"foreignKey:UserID" json:"players"`
	Notifications   []Notification `gorm:"foreignKey:UserID" json:"notifications"`
}

func (User) TableName() string {
	return "users"
}

type PasswordResetToken struct {
	Email     string     `gorm:"primaryKey;column:email" json:"email"`
	Token     string     `gorm:"column:token;not null" json:"token"`
	CreatedAt *time.Time `gorm:"column:created_at" json:"created_at"`
}

func (PasswordResetToken) TableName() string {
	return "password_reset_tokens"
}

type Session struct {
	ID           uuid.UUID `gorm:"primaryKey;column:id" json:"id"`
	UserID       uuid.UUID `gorm:"column:user_id" json:"user_id"`
	IPAddress    string    `gorm:"column:ip_address;size:45" json:"ip_address"`
	UserAgent    string    `gorm:"column:user_agent;type:text" json:"user_agent"`
	Payload      string    `gorm:"column:payload;type:text;not null" json:"payload"`
	LastActivity int       `gorm:"column:last_activity;not null;index" json:"last_activity"`
	User         *User     `gorm:"foreignKey:UserID" json:"user"`
}

func (Session) TableName() string {
	return "sessions"
}

type Lobby struct {
	ID               uuid.UUID         `gorm:"primaryKey;column:id" json:"id"`
	Name             string            `gorm:"column:name;not null;index" json:"name"`
	OwnerID          uuid.UUID         `gorm:"column:owner_id;not null" json:"owner_id"`
	Owner            User              `gorm:"foreignKey:OwnerID" json:"owner"`
	Type             string            `gorm:"column:type;type:varchar(20);default:'public';not null" json:"type"`
	Status           string            `gorm:"column:status;type:varchar(20);default:'waiting';not null;index" json:"status"`
	MaxPlayers       int               `gorm:"column:max_players;default:4;not null" json:"max_players"`
	CurrentPlayers   int               `gorm:"column:current_players;default:0;not null" json:"current_players"`
	PrivacyLevel     string            `gorm:"column:privacy_level;type:varchar(20);default:'open';not null" json:"privacy_level"`
	PasswordHash     *string           `gorm:"column:password_hash" json:"password_hash"`
	SpectatorAllowed bool              `gorm:"column:spectator_allowed;default:true;not null" json:"spectator_allowed"`
	SpectatorCount   int               `gorm:"column:spectator_count;default:0;not null" json:"spectator_count"`
	GameMode         string            `gorm:"column:game_mode;type:varchar(20);default:'casual';not null" json:"game_mode"`
	GameSettings     json.RawMessage   `gorm:"column:game_settings;type:jsonb" json:"game_settings"`
	CreatedAt        time.Time         `gorm:"column:created_at" json:"created_at"`
	UpdatedAt        time.Time         `gorm:"column:updated_at" json:"updated_at"`
	LobbyInvitations []LobbyInvitation `gorm:"foreignKey:LobbyID" json:"invitations"`
	Games            []Game            `gorm:"foreignKey:LobbyID" json:"games"`
	Players          []Player          `gorm:"foreignKey:LobbyID" json:"players"`
	LobbyQueues      []LobbyQueue      `gorm:"foreignKey:LobbyID" json:"lobby_queues"`
}

func (Lobby) TableName() string {
	return "lobbies"
}

type Game struct {
	ID                  uuid.UUID `gorm:"primaryKey;column:id" json:"id"`
	LobbyID             uuid.UUID `gorm:"column:lobby_id" json:"lobby_id"`
	Lobby               Lobby     `gorm:"foreignKey:LobbyID" json:"lobby"`
	Status              string    `gorm:"column:status;type:varchar(20);default:'waiting';not null" json:"status"`
	CurrentTurnPlayerID *uuid.UUID `gorm:"column:current_turn_player_id;null" json:"current_turn_player_id"`
	RoundNumber         int       `gorm:"column:round_number;default:1;not null" json:"round_number"`
	Winner              string    `gorm:"column:winner;type:varchar(20);default:'none';not null" json:"winner"`
	CreatedAt           time.Time `gorm:"column:created_at" json:"created_at"`
	UpdatedAt           time.Time `gorm:"column:updated_at" json:"updated_at"`
	Decks               []Deck    `gorm:"foreignKey:GameID" json:"decks"`
	Cards               []Card    `gorm:"foreignKey:GameID" json:"cards"`
	Players             []Player  `gorm:"foreignKey:GameID" json:"players"`
}

func (Game) TableName() string {
	return "games"
}

type LobbyInvitation struct {
	ID              uuid.UUID  `gorm:"primaryKey;column:id" json:"id"`
	LobbyID         uuid.UUID  `gorm:"column:lobby_id;not null" json:"lobby_id"`
	Lobby           Lobby      `gorm:"foreignKey:LobbyID" json:"lobby"`
	InviterID       uuid.UUID  `gorm:"column:inviter_id;not null" json:"inviter_id"`
	Inviter         User       `gorm:"foreignKey:InviterID" json:"inviter"`
	InvitedUserID   uuid.UUID  `gorm:"column:invited_user_id;not null" json:"invited_user_id"`
	InvitedUser     User       `gorm:"foreignKey:InvitedUserID" json:"invited_user"`
	Status          string     `gorm:"column:status;type:varchar(20);default:'pending';not null;index" json:"status"`
	ExpiresAt       time.Time  `gorm:"column:expires_at;not null;index" json:"expires_at"`
	CreatedAt       *time.Time `gorm:"column:created_at" json:"created_at"`
	UpdatedAt       *time.Time `gorm:"column:updated_at" json:"updated_at"`
}

func (LobbyInvitation) TableName() string {
	return "lobby_invitations"
}

type Deck struct {
	ID                uuid.UUID       `gorm:"primaryKey;column:id" json:"id"`
	GameID            uuid.UUID       `gorm:"column:game_id;not null" json:"game_id"`
	Game              Game            `gorm:"foreignKey:GameID" json:"game"`
	DeckType          string          `gorm:"column:deck_type;type:varchar(20);default:'standard';not null" json:"deck_type"`
	TotalCards        int             `gorm:"column:total_cards;default:52;not null" json:"total_cards"`
	RemainingCards    int             `gorm:"column:remaining_cards;default:52;not null" json:"remaining_cards"`
	ExternalDeckID    string          `gorm:"column:external_deck_id" json:"external_deck_id"`
	DeckConfiguration json.RawMessage `gorm:"column:deck_configuration;type:jsonb" json:"deck_configuration"`
	CreatedAt         time.Time       `gorm:"column:created_at" json:"created_at"`
	UpdatedAt         time.Time       `gorm:"column:updated_at" json:"updated_at"`
	Cards             []Card          `gorm:"foreignKey:DeckID" json:"cards"`
}

func (Deck) TableName() string {
	return "decks"
}

type Card struct {
	ID            uuid.UUID  `gorm:"primaryKey;column:id" json:"id"`
	DeckID        uuid.UUID  `gorm:"column:deck_id;not null" json:"deck_id"`
	Deck          Deck       `gorm:"foreignKey:DeckID" json:"deck"`
	GameID        uuid.UUID  `gorm:"column:game_id;not null" json:"game_id"`
	Game          Game       `gorm:"foreignKey:GameID" json:"game"`
	Code          string     `gorm:"column:code;unique;not null;size:10" json:"code"`
	Value         string     `gorm:"column:value;size:10;not null" json:"value"`
	Suit          string     `gorm:"column:suit;size:10;not null" json:"suit"`
	ImageURL      *string    `gorm:"column:image_url" json:"image_url"`
	Status        string     `gorm:"column:status;type:varchar(20);default:'in_deck';not null" json:"status"`
	LocationType  string     `gorm:"column:location_type;type:varchar(20);default:'deck';not null" json:"location_type"`
	PlayerID      *uuid.UUID `gorm:"column:player_id" json:"player_id"`
	Player        *User      `gorm:"foreignKey:PlayerID" json:"player"`
	IsSpecialCard bool       `gorm:"column:is_special_card;default:false;not null" json:"is_special_card"`
	SpecialAction string     `gorm:"column:special_action;type:varchar(20);default:'none';not null" json:"special_action"`
	CreatedAt     *time.Time `gorm:"column:created_at" json:"created_at"`
	UpdatedAt     *time.Time `gorm:"column:updated_at" json:"updated_at"`
}

func (Card) TableName() string {
	return "cards"
}

type Player struct {
	ID        uuid.UUID  `gorm:"primaryKey;column:id" json:"id"`
	GameID    uuid.UUID  `gorm:"column:game_id;not null" json:"game_id"`
	UserID    uuid.UUID  `gorm:"column:user_id;not null" json:"user_id"`
	LobbyID   uuid.UUID  `gorm:"column:lobby_id;not null" json:"lobby_id"`
	Role      string     `gorm:"column:role;type:varchar(20);default:'player1';not null" json:"role"`
	IsReady   bool       `gorm:"column:is_ready;default:false;not null" json:"is_ready"`
	Score     int        `gorm:"column:score;default:0;not null" json:"score"`
	CreatedAt *time.Time `gorm:"column:created_at" json:"created_at"`
	UpdatedAt *time.Time `gorm:"column:updated_at" json:"updated_at"`

	User      User       `gorm:"foreignKey:UserID" json:"user"`
	Lobby     Lobby      `gorm:"foreignKey:LobbyID" json:"lobby"`
	Game      Game       `gorm:"foreignKey:GameID" json:"game"`
}

func (Player) TableName() string {
	return "players"
}

type LobbyQueue struct {
	ID        uuid.UUID  `gorm:"primaryKey;column:id" json:"id"`
	LobbyID   uuid.UUID  `gorm:"column:lobby_id;not null" json:"lobby_id"`
	Lobby     Lobby      `gorm:"foreignKey:LobbyID" json:"lobby"`
	UserID    uuid.UUID  `gorm:"column:user_id;not null" json:"user_id"`
	User      User       `gorm:"foreignKey:UserID" json:"user"`
	QueueType string     `gorm:"column:queue_type;type:varchar(20);default:'waiting';not null" json:"queue_type"`
	Priority  int        `gorm:"column:priority;default:0;not null" json:"priority"`
	Position  *int       `gorm:"column:position" json:"position"`
	CreatedAt *time.Time `gorm:"column:created_at" json:"created_at"`
	UpdatedAt *time.Time `gorm:"column:updated_at" json:"updated_at"`
}

func (LobbyQueue) TableName() string {
	return "lobby_queues"
}

type Notification struct {
	ID        uuid.UUID `gorm:"type:uuid;primaryKey;column:id" json:"id"`
	Type      *string   `gorm:"column:type" json:"type"`
	UserID    uuid.UUID `gorm:"column:user_id;not null" json:"user_id"`
	Data      json.RawMessage    `gorm:"column:data;type:json;not null" json:"data"`
	ReadAt    time.Time `gorm:"column:read_at" json:"read_at"`
	CreatedAt time.Time `gorm:"column:created_at" json:"created_at"`
	UpdatedAt time.Time `gorm:"column:updated_at" json:"updated_at"`
	User      User      `gorm:"foreignKey:UserID" json:"user"`
}

func (Notification) TableName() string {
	return "notifications"
}

type PersonalAccessToken struct {
	ID            uuid.UUID  `gorm:"primaryKey;column:id" json:"id"`
	TokenableType string     `gorm:"column:tokenable_type;not null" json:"tokenable_type"`
	TokenableID   uuid.UUID  `gorm:"column:tokenable_id;not null" json:"tokenable_id"`
	Name          string     `gorm:"column:name;not null" json:"name"`
	Token         string     `gorm:"column:token;unique;not null;size:64" json:"token"`
	Abilities     *string    `gorm:"column:abilities;type:text" json:"abilities"`
	LastUsedAt    *time.Time `gorm:"column:last_used_at" json:"last_used_at"`
	ExpiresAt     *time.Time `gorm:"column:expires_at" json:"expires_at"`
	CreatedAt     *time.Time `gorm:"column:created_at" json:"created_at"`
	UpdatedAt     *time.Time `gorm:"column:updated_at" json:"updated_at"`
}

func (PersonalAccessToken) TableName() string {
	return "personal_access_tokens"
}
