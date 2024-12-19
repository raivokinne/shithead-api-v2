-- +goose up
CREATE TABLE lobby_invitations (
    id UUID PRIMARY KEY,
    lobby_id UUID NOT NULL,
    game_id UUID NULL,
    inviter_id UUID NOT NULL,
    invited_user_id UUID NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'pending',
    invitation_token VARCHAR(255) UNIQUE NOT NULL,
    expires_at TIMESTAMP NOT NULL,
    created_at TIMESTAMP NULL,
    updated_at TIMESTAMP NULL,

    FOREIGN KEY (lobby_id) REFERENCES lobbies(id) ON DELETE CASCADE,
    FOREIGN KEY (game_id) REFERENCES games(id) ON DELETE SET NULL,
    FOREIGN KEY (inviter_id) REFERENCES users(id) ON DELETE CASCADE,
    FOREIGN KEY (invited_user_id) REFERENCES users(id) ON DELETE CASCADE
);

CREATE INDEX idx_lobby_invitations_status ON lobby_invitations(status);
CREATE INDEX idx_lobby_invitations_expires_at ON lobby_invitations(expires_at);

-- +goose down
DROP TABLE IF EXISTS lobby_invitations;
