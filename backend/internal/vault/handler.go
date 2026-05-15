// Package vault serves the encrypted vault entry endpoints.
package vault

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/i-jurian/legavi/backend/internal/auth"
	"github.com/i-jurian/legavi/backend/internal/respond"
	"github.com/i-jurian/legavi/backend/internal/store"
)

const (
	defaultListLimit = 100
	maxListLimit     = 500
)

type Handler struct {
	store *store.Store
}

func NewHandler(s *store.Store) *Handler {
	return &Handler{store: s}
}

type entryRequest struct {
	Preview             []byte   `json:"preview"`
	Bundle              []byte   `json:"bundle"`
	SortOrder           int32    `json:"sortOrder"`
	RecipientContactIDs []string `json:"recipientContactIds"`
}

type entryPreview struct {
	ID            string  `json:"id"`
	Preview       []byte  `json:"preview"`
	SortOrder     int32   `json:"sortOrder"`
	SchemaVersion int16   `json:"schemaVersion"`
	CreatedAt     string  `json:"createdAt"`
	UpdatedAt     string  `json:"updatedAt"`
	DeletedAt     *string `json:"deletedAt"`
}

type entryDetail struct {
	entryPreview
	Bundle []byte `json:"bundle"`
}

type listResponse struct {
	Entries    []entryPreview `json:"entries"`
	NextCursor *string        `json:"nextCursor"`
}

func (h *Handler) List(w http.ResponseWriter, r *http.Request) *respond.Error {
	userID, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		return respond.Unauthorized("unauthorized")
	}

	limit := int32(defaultListLimit)
	if q := r.URL.Query().Get("limit"); q != "" {
		n, err := strconv.Atoi(q)
		if err != nil || n <= 0 {
			return respond.BadRequest("invalid limit", err)
		}
		if n > maxListLimit {
			n = maxListLimit
		}
		limit = int32(n)
	}

	includeDeleted := r.URL.Query().Get("includeDeleted") == "true"

	rows, err := h.store.ListUserVaultEntries(r.Context(), userID, includeDeleted, limit)
	if err != nil {
		return respond.Internal(fmt.Errorf("list entries: %w", err))
	}

	entries := make([]entryPreview, len(rows))
	for i, row := range rows {
		entries[i] = toEntryPreview(row)
	}

	respond.JSON(w, http.StatusOK, listResponse{Entries: entries, NextCursor: nil})
	return nil
}

func (h *Handler) Create(w http.ResponseWriter, r *http.Request) *respond.Error {
	userID, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		return respond.Unauthorized("unauthorized")
	}

	req, aerr := decodeEntryRequest(r)
	if aerr != nil {
		return aerr
	}

	entry, err := h.store.CreateVaultEntry(r.Context(), userID, req.Preview, req.Bundle, req.SortOrder)
	if err != nil {
		return respond.Internal(fmt.Errorf("create entry: %w", err))
	}

	respond.JSON(w, http.StatusCreated, toEntryDetail(entry))
	return nil
}

func (h *Handler) Get(w http.ResponseWriter, r *http.Request) *respond.Error {
	userID, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		return respond.Unauthorized("unauthorized")
	}

	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		return respond.BadRequest("invalid id")
	}

	entry, err := h.store.GetVaultEntry(r.Context(), id, userID)
	if errors.Is(err, store.ErrNotFound) {
		return respond.NotFound(err)
	}
	if err != nil {
		return respond.Internal(fmt.Errorf("get entry: %w", err))
	}

	respond.JSON(w, http.StatusOK, toEntryDetail(entry))
	return nil
}

func (h *Handler) Update(w http.ResponseWriter, r *http.Request) *respond.Error {
	userID, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		return respond.Unauthorized("unauthorized")
	}

	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		return respond.BadRequest("invalid id")
	}

	req, aerr := decodeEntryRequest(r)
	if aerr != nil {
		return aerr
	}

	entry, err := h.store.UpdateVaultEntry(r.Context(), id, userID, req.Preview, req.Bundle, req.SortOrder)
	if errors.Is(err, store.ErrNotFound) {
		return respond.NotFound(err)
	}
	if err != nil {
		return respond.Internal(fmt.Errorf("update entry: %w", err))
	}

	respond.JSON(w, http.StatusOK, toEntryDetail(entry))
	return nil
}

func (h *Handler) SoftDelete(w http.ResponseWriter, r *http.Request) *respond.Error {
	userID, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		return respond.Unauthorized("unauthorized")
	}

	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		return respond.BadRequest("invalid id")
	}

	_, err = h.store.SoftDeleteVaultEntry(r.Context(), id, userID)
	if errors.Is(err, store.ErrNotFound) {
		return respond.NotFound(err)
	}
	if err != nil {
		return respond.Internal(fmt.Errorf("delete entry: %w", err))
	}

	w.WriteHeader(http.StatusNoContent)
	return nil
}

func (h *Handler) Restore(w http.ResponseWriter, r *http.Request) *respond.Error {
	userID, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		return respond.Unauthorized("unauthorized")
	}

	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		return respond.BadRequest("invalid id")
	}

	entry, err := h.store.RestoreVaultEntry(r.Context(), id, userID)
	if errors.Is(err, store.ErrNotFound) {
		return respond.New(http.StatusNotFound, "cannot restore: not found, not deleted, or window expired", err)
	}
	if err != nil {
		return respond.Internal(fmt.Errorf("restore entry: %w", err))
	}

	respond.JSON(w, http.StatusOK, toEntryDetail(entry))
	return nil
}

func decodeEntryRequest(r *http.Request) (entryRequest, *respond.Error) {
	var req entryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return req, respond.BadRequest("invalid json", err)
	}
	if len(req.Preview) == 0 || len(req.Bundle) == 0 {
		return req, respond.BadRequest("preview and bundle required")
	}
	if len(req.RecipientContactIDs) > 0 {
		return req, respond.BadRequest("recipient assignment not yet supported")
	}
	return req, nil
}

func toEntryPreview(row store.ListUserVaultEntriesRow) entryPreview {
	return entryPreview{
		ID:            uuid.UUID(row.ID.Bytes).String(),
		Preview:       row.Preview,
		SortOrder:     row.SortOrder,
		SchemaVersion: row.SchemaVersion,
		CreatedAt:     row.CreatedAt.Time.UTC().Format(time.RFC3339),
		UpdatedAt:     row.UpdatedAt.Time.UTC().Format(time.RFC3339),
		DeletedAt:     nullableTime(row.DeletedAt),
	}
}

func toEntryDetail(e store.VaultEntry) entryDetail {
	return entryDetail{
		entryPreview: entryPreview{
			ID:            uuid.UUID(e.ID.Bytes).String(),
			Preview:       e.Preview,
			SortOrder:     e.SortOrder,
			SchemaVersion: e.SchemaVersion,
			CreatedAt:     e.CreatedAt.Time.UTC().Format(time.RFC3339),
			UpdatedAt:     e.UpdatedAt.Time.UTC().Format(time.RFC3339),
			DeletedAt:     nullableTime(e.DeletedAt),
		},
		Bundle: e.Bundle,
	}
}

func nullableTime(t pgtype.Timestamptz) *string {
	if !t.Valid {
		return nil
	}
	s := t.Time.UTC().Format(time.RFC3339)
	return &s
}
