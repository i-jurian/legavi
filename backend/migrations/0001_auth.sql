-- +goose Up
-- +goose StatementBegin
CREATE TABLE users (
    id           UUID PRIMARY KEY,
    email        TEXT NOT NULL UNIQUE,
    display_name TEXT NOT NULL,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at   TIMESTAMPTZ
);

CREATE INDEX users_email_active_idx ON users(email) WHERE deleted_at IS NULL;

CREATE TABLE credentials (
    id            BYTEA PRIMARY KEY,
    user_id       UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    public_key    BYTEA NOT NULL,
    sign_count    BIGINT NOT NULL DEFAULT 0,
    transports    TEXT[],
    aaguid        UUID,
    age_recipient TEXT NOT NULL,
    nickname      TEXT NOT NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_used_at  TIMESTAMPTZ,
    deleted_at    TIMESTAMPTZ
);

CREATE INDEX credentials_user_active_idx ON credentials(user_id) WHERE deleted_at IS NULL; 

CREATE TABLE webauthn_sessions (
    id           UUID PRIMARY KEY,
    user_id      UUID REFERENCES users(id) ON DELETE CASCADE,
    session_data BYTEA NOT NULL,
    purpose      TEXT NOT NULL CHECK (purpose IN ('register', 'login')),
    expires_at   TIMESTAMPTZ NOT NULL,
    consumed_at  TIMESTAMPTZ
);

CREATE INDEX webauthn_sessions_expires_idx ON webauthn_sessions(expires_at); 
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE webauthn_sessions;
DROP TABLE credentials;
DROP TABLE users;
-- +goose StatementEnd
