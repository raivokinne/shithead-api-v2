-- +goose up
CREATE TABLE lobby_queues (
    id UUID PRIMARY KEY,
    lobby_id UUID NOT NULL,
    user_id UUID NOT NULL,
    queue_type VARCHAR(20) NOT NULL DEFAULT 'waiting',
    priority INTEGER NOT NULL DEFAULT 0,
    position INTEGER NULL,
    created_at TIMESTAMP NULL,
    updated_at TIMESTAMP NULL,

    FOREIGN KEY (lobby_id) REFERENCES lobbies(id) ON DELETE CASCADE,
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);

-- +goose down
DROP TABLE IF EXISTS lobby_queues;
