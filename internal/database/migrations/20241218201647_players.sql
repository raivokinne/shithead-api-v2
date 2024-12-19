-- +goose up
CREATE TABLE players (
    id UUID PRIMARY KEY,
    game_id UUID NOT NULL,
    user_id UUID NOT NULL,
    lobby_id UUID NOT NULL,
    role VARCHAR(20) NOT NULL DEFAULT 'player1',
    is_ready BOOLEAN NOT NULL DEFAULT false,
    score INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMP NULL,
    updated_at TIMESTAMP NULL,

    FOREIGN KEY (game_id) REFERENCES games(id) ON DELETE CASCADE,
    FOREIGN KEY (user_id) REFERENCES users(id),
    FOREIGN KEY (lobby_id) REFERENCES lobbies(id) ON DELETE CASCADE
);

-- +goose down
DROP TABLE IF EXISTS players;
