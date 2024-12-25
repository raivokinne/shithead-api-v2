-- +goose up
CREATE TABLE notifications (
    id UUID PRIMARY KEY,
    type VARCHAR(255) NULL,
    user_id UUID NOT NULL,
    data JSON NOT NULL,
    read_at TIMESTAMP NULL,
    created_at TIMESTAMP NULL,
    updated_at TIMESTAMP NULL,

    FOREIGN KEY(user_id) REFERENCES users(id)
);

-- +goose down
DROP TABLE IF EXISTS notifications;
