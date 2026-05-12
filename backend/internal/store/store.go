// Package store wraps the sqlc-generated queries with a domain-friendly facade.
package store

import (
	"context"
	"strings"
	"time"

	"github.com/go-webauthn/webauthn/webauthn"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Store struct {
	*Queries
	Pool *pgxpool.Pool
}

func NewStore(pool *pgxpool.Pool) *Store {
	return &Store{Queries: New(pool), Pool: pool}
}

func (s *Store) CreateUser(ctx context.Context, email, displayName string) (User, error) {
	return s.Queries.CreateUser(ctx, CreateUserParams{
		ID:          pgtype.UUID{Bytes: uuid.New(), Valid: true},
		Email:       strings.ToLower(strings.TrimSpace(email)),
		DisplayName: strings.TrimSpace(displayName),
	})
}

func (s *Store) CreateSession(
	ctx context.Context,
	userID uuid.UUID,
	sessionData []byte,
	purpose string,
	ttl time.Duration,
) (WebauthnSession, error) {
	return s.Queries.CreateSession(ctx, CreateSessionParams{
		ID:          pgtype.UUID{Bytes: uuid.New(), Valid: true},
		UserID:      pgtype.UUID{Bytes: userID, Valid: true},
		SessionData: sessionData,
		Purpose:     purpose,
		ExpiresAt:   pgtype.Timestamptz{Time: time.Now().Add(ttl), Valid: true},
	})
}

func (s *Store) ConsumeSession(
	ctx context.Context,
	id uuid.UUID,
	purpose string,
) (WebauthnSession, error) {
	return s.Queries.ConsumeSession(ctx, ConsumeSessionParams{
		ID:      pgtype.UUID{Bytes: id, Valid: true},
		Purpose: purpose,
	})
}

func (s *Store) CreateCredential(
	ctx context.Context,
	userID uuid.UUID,
	cred *webauthn.Credential,
	ageRecipient string,
	nickname *string,
) (Credential, error) {
	transports := make([]string, len(cred.Transport))
	for i, t := range cred.Transport {
		transports[i] = string(t)
	}

	return s.Queries.CreateCredential(ctx, CreateCredentialParams{
		ID:           cred.ID,
		PublicKey:    cred.PublicKey,
		SignCount:    int64(cred.Authenticator.SignCount),
		UserID:       pgtype.UUID{Bytes: userID, Valid: true},
		Aaguid:       pgtype.UUID{Bytes: [16]byte(cred.Authenticator.AAGUID), Valid: true},
		Transports:   transports,
		AgeRecipient: ageRecipient,
		Nickname:     nickname,
	})
}
