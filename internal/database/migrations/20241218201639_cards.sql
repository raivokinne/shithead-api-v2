-- +goose up
CREATE TABLE cards (
    id UUID PRIMARY KEY,
    deck_id UUID NOT NULL,
    game_id UUID NOT NULL,
    code VARCHAR(10) NOT NULL,
    value VARCHAR(10) NOT NULL,
    suit VARCHAR(10) NOT NULL,
    image_url VARCHAR(255) NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'in_deck',
    location_type VARCHAR(20) NOT NULL DEFAULT 'deck',
    player_id UUID NULL,
    is_special_card BOOLEAN NOT NULL DEFAULT false,
    special_action VARCHAR(20) NOT NULL DEFAULT 'none',
    created_at TIMESTAMP NULL,
    updated_at TIMESTAMP NULL,

    FOREIGN KEY (deck_id) REFERENCES decks(id) ON DELETE CASCADE,
    FOREIGN KEY (game_id) REFERENCES games(id) ON DELETE CASCADE,
    FOREIGN KEY (player_id) REFERENCES players(id) ON DELETE SET NULL
);

-- +goose down
DROP TABLE IF EXISTS cards;
