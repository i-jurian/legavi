// Package store wraps the sqlc-generated queries with a domain-friendly facade.
package store

import (
	"context"
	"strings"
	"time"

	"github.com/go-webauthn/webauthn/protocol"
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

func (s *Store) GetUser(ctx context.Context, id uuid.UUID) (User, error) {
	return s.GetUserByID(ctx, pgtype.UUID{Bytes: id, Valid: true})
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
	nickname string,
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

func (s *Store) ListUserWebAuthnCredentials(ctx context.Context, userID uuid.UUID) ([]webauthn.Credential, error) {
	rows, err := s.ListUserCredentials(ctx, pgtype.UUID{Bytes: userID, Valid: true})
	if err != nil {
		return nil, err
	}
	out := make([]webauthn.Credential, len(rows))
	for i, row := range rows {
		transports := make([]protocol.AuthenticatorTransport, len(row.Transports))
		for j, t := range row.Transports {
			transports[j] = protocol.AuthenticatorTransport(t)
		}
		aaguid := row.Aaguid.Bytes
		out[i] = webauthn.Credential{
			ID:        row.ID,
			PublicKey: row.PublicKey,
			Transport: transports,
			Authenticator: webauthn.Authenticator{
				AAGUID:    aaguid[:],
				SignCount: uint32(row.SignCount),
			},
		}
	}
	return out, nil
}

func (s *Store) UpdateCredentialUsage(ctx context.Context, credentialID []byte, signCount uint32) error {
	return s.Queries.UpdateCredentialUsage(ctx, UpdateCredentialUsageParams{
		ID:        credentialID,
		SignCount: int64(signCount),
	})
}

func (s *Store) CreateVaultEntry(
	ctx context.Context,
	userID uuid.UUID,
	preview, bundle []byte,
	sortOrder int32,
) (VaultEntry, error) {
	return s.Queries.CreateVaultEntry(ctx, CreateVaultEntryParams{
		ID:        pgtype.UUID{Bytes: uuid.New(), Valid: true},
		UserID:    pgtype.UUID{Bytes: userID, Valid: true},
		Preview:   preview,
		Bundle:    bundle,
		SortOrder: sortOrder,
	})
}

func (s *Store) GetVaultEntry(ctx context.Context, id, userID uuid.UUID) (VaultEntry, error) {
	return s.Queries.GetVaultEntry(ctx, GetVaultEntryParams{
		ID:     pgtype.UUID{Bytes: id, Valid: true},
		UserID: pgtype.UUID{Bytes: userID, Valid: true},
	})
}

func (s *Store) ListUserVaultEntries(
	ctx context.Context,
	userID uuid.UUID,
	includeDeleted bool,
	limit int32,
) ([]ListUserVaultEntriesRow, error) {
	return s.Queries.ListUserVaultEntries(ctx, ListUserVaultEntriesParams{
		UserID:         pgtype.UUID{Bytes: userID, Valid: true},
		IncludeDeleted: includeDeleted,
		RowLimit:       limit,
	})
}

func (s *Store) UpdateVaultEntry(
	ctx context.Context,
	id, userID uuid.UUID,
	preview, bundle []byte,
	sortOrder int32,
) (VaultEntry, error) {
	return s.Queries.UpdateVaultEntry(ctx, UpdateVaultEntryParams{
		ID:        pgtype.UUID{Bytes: id, Valid: true},
		UserID:    pgtype.UUID{Bytes: userID, Valid: true},
		Preview:   preview,
		Bundle:    bundle,
		SortOrder: sortOrder,
	})
}

func (s *Store) SoftDeleteVaultEntry(ctx context.Context, id, userID uuid.UUID) (VaultEntry, error) {
	return s.Queries.SoftDeleteVaultEntry(ctx, SoftDeleteVaultEntryParams{
		ID:     pgtype.UUID{Bytes: id, Valid: true},
		UserID: pgtype.UUID{Bytes: userID, Valid: true},
	})
}

func (s *Store) RestoreVaultEntry(ctx context.Context, id, userID uuid.UUID) (VaultEntry, error) {
	return s.Queries.RestoreVaultEntry(ctx, RestoreVaultEntryParams{
		ID:     pgtype.UUID{Bytes: id, Valid: true},
		UserID: pgtype.UUID{Bytes: userID, Valid: true},
	})
}
