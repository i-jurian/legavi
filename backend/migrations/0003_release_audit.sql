-- +goose Up
-- +goose StatementBegin
CREATE TABLE release_state (
    user_id             UUID PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    state               TEXT NOT NULL CHECK (state IN (
                            'ACTIVE',
                            'REMINDED_SOFT',
                            'REMINDED_FIRM',
                            'REMINDED_FINAL',
                            'COOLING',
                            'FINAL_HOLD',
                            'RELEASED'
                        )) DEFAULT 'ACTIVE',
    last_checkin_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    state_entered_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    cooling_started_at  TIMESTAMPTZ,
    final_hold_until    TIMESTAMPTZ,
    is_false_positive BOOLEAN NOT NULL DEFAULT false
);

CREATE TABLE release_offsets (
    user_id          UUID PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    soft_after_days  INTEGER NOT NULL DEFAULT 7,
    firm_after_days  INTEGER NOT NULL DEFAULT 14,
    final_after_days INTEGER NOT NULL DEFAULT 30,
    cooling_hours    INTEGER NOT NULL DEFAULT 48,
    final_hold_hours INTEGER NOT NULL DEFAULT 24
);

CREATE TABLE audit_log (
    id              UUID PRIMARY KEY,
    user_id         UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    sequence        BIGINT NOT NULL,
    event_type      TEXT NOT NULL,
    payload         JSONB NOT NULL,
    prev_entry_hash BYTEA NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (user_id, sequence)
);

CREATE INDEX audit_log_user_sequence_idx ON audit_log(user_id, sequence);

CREATE TABLE audit_checkpoints (
    id              UUID PRIMARY KEY,
    user_id         UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    sequence        BIGINT NOT NULL,
    chain_head_hash BYTEA NOT NULL,
    signature       BYTEA NOT NULL,
    signed_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (user_id, sequence)
);

CREATE INDEX audit_checkpoints_user_sequence_idx ON audit_checkpoints(user_id, sequence DESC);

CREATE TABLE jobs (
    id           UUID PRIMARY KEY,
    type         TEXT NOT NULL,
    payload      JSONB NOT NULL,
    run_after    TIMESTAMPTZ NOT NULL DEFAULT now(),
    attempts     INTEGER NOT NULL DEFAULT 0,
    max_attempts INTEGER NOT NULL DEFAULT 5,
    status       TEXT NOT NULL CHECK (status IN ('pending', 'running', 'completed', 'failed'))
                     DEFAULT 'pending',
    locked_by    TEXT,
    locked_at    TIMESTAMPTZ,
    completed_at TIMESTAMPTZ,
    last_error   TEXT,
    dedup_key    TEXT,
    UNIQUE (dedup_key)
);

CREATE INDEX jobs_pending_idx ON jobs(run_after) WHERE status = 'pending';
CREATE INDEX jobs_locked_idx ON jobs(locked_by) WHERE status = 'running';
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE jobs;
DROP TABLE audit_checkpoints;
DROP TABLE audit_log;
DROP TABLE release_offsets;
DROP TABLE release_state;
-- +goose StatementEnd
