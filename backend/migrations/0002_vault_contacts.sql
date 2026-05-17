-- +goose Up
-- +goose StatementBegin
CREATE TABLE vault_entries (
    id             UUID PRIMARY KEY,
    user_id        UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    preview        BYTEA NOT NULL,
    bundle         BYTEA NOT NULL,
    sort_order     INTEGER NOT NULL DEFAULT 0,
    schema_version SMALLINT NOT NULL DEFAULT 1,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at     TIMESTAMPTZ
);

CREATE INDEX vault_entries_user_active_idx ON vault_entries(user_id, sort_order)
    WHERE deleted_at IS NULL;

CREATE TABLE contacts (
    id               UUID PRIMARY KEY,
    user_id          UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    email            TEXT NOT NULL,
    display_name     TEXT NOT NULL,
    status           TEXT NOT NULL CHECK (status IN ('pending', 'verified', 'removed'))
                         DEFAULT 'pending',
    contact_user_id  UUID REFERENCES users(id) ON DELETE SET NULL,
    age_recipient    TEXT,
    fingerprint_hash BYTEA,
    invited_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    verified_at      TIMESTAMPTZ,
    removed_at       TIMESTAMPTZ
);

CREATE INDEX contacts_user_status_idx ON contacts(user_id, status);
CREATE INDEX contacts_email_user_idx ON contacts(user_id, email);

CREATE TABLE entry_recipients (
    entry_id    UUID NOT NULL REFERENCES vault_entries(id) ON DELETE CASCADE,
    contact_id  UUID NOT NULL REFERENCES contacts(id) ON DELETE CASCADE,
    assigned_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (entry_id, contact_id)
);

CREATE INDEX entry_recipients_contact_idx ON entry_recipients(contact_id);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE entry_recipients;
DROP TABLE contacts;
DROP TABLE vault_entries;
-- +goose StatementEnd
