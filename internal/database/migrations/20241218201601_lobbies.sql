-- +goose up
CREATE TABLE lobbies (
    id UUID PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    owner_id UUID NOT NULL,
    type VARCHAR(20) NOT NULL DEFAULT 'public',
    status VARCHAR(20) NOT NULL DEFAULT 'waiting',
    max_players INTEGER NOT NULL DEFAULT 4,
    current_players INTEGER NOT NULL DEFAULT 0,
    privacy_level VARCHAR(20) NOT NULL DEFAULT 'open',
    password_hash VARCHAR(255) NULL,
    spectator_allowed BOOLEAN NOT NULL DEFAULT true,
    spectator_count INTEGER NOT NULL DEFAULT 0,
    game_mode VARCHAR(20) NOT NULL DEFAULT 'casual',
    game_settings JSONB NULL,
    created_at TIMESTAMP NULL,
    updated_at TIMESTAMP NULL,

    FOREIGN KEY (owner_id) REFERENCES users(id) ON DELETE CASCADE
);

CREATE INDEX idx_lobbies_name ON lobbies(name);
CREATE INDEX idx_lobbies_status ON lobbies(status);

-- +goose down
DROP TABLE IF EXISTS lobbies;
