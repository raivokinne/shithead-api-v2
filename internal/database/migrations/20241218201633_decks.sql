-- +goose up
CREATE TABLE decks (
    id UUID PRIMARY KEY,
    game_id UUID NOT NULL,
    deck_type VARCHAR(20) NOT NULL DEFAULT 'standard',
    total_cards INTEGER NOT NULL DEFAULT 52,
    remaining_cards INTEGER NOT NULL DEFAULT 52,
    external_deck_id VARCHAR(255) NULL,
    deck_configuration JSONB NULL,
    created_at TIMESTAMP NULL,
    updated_at TIMESTAMP NULL,

    FOREIGN KEY (game_id) REFERENCES games(id) ON DELETE CASCADE
);

-- +goose down
DROP TABLE IF EXISTS decks;
