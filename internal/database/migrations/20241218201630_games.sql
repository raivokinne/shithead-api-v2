-- +goose up
CREATE TABLE games (
    id UUID PRIMARY KEY,
    lobby_id UUID NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'waiting',
    current_turn_player_id UUID NULL,
    round_number INTEGER NOT NULL DEFAULT 1,
    winner VARCHAR(20) NOT NULL DEFAULT 'none',
    created_at TIMESTAMP NULL,
    updated_at TIMESTAMP NULL,

    FOREIGN KEY (lobby_id) REFERENCES lobbies(id) ON DELETE CASCADE
);

-- +goose down
DROP TABLE IF EXISTS games;
