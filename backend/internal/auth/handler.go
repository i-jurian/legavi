package auth

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/webauthn"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/i-jurian/legavi/backend/internal/store"
)

type Handler struct {
	webauthn *WebAuthn
	jwt      *JWT
	store    *store.Store
}

type webauthnUser struct {
	id          []byte
	email       string
	displayName string
	credentials []webauthn.Credential
}

type registerStartRequest struct {
	Email       string `json:"email"`
	DisplayName string `json:"displayName"`
}

type registerVerifyRequest struct {
	AgeRecipient string          `json:"ageRecipient"`
	Nickname     *string         `json:"nickname,omitempty"`
	Response     json.RawMessage `json:"response"`
}

func NewHandler(wa *WebAuthn, j *JWT, s *store.Store) *Handler {
	return &Handler{webauthn: wa, jwt: j, store: s}
}

func (u *webauthnUser) WebAuthnID() []byte                         { return u.id }
func (u *webauthnUser) WebAuthnName() string                       { return u.email }
func (u *webauthnUser) WebAuthnDisplayName() string                { return u.displayName }
func (u *webauthnUser) WebAuthnCredentials() []webauthn.Credential { return u.credentials }

func (h *Handler) RegisterStart(w http.ResponseWriter, r *http.Request) {
	// Parse then validate body
	var req registerStartRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	email := strings.ToLower(strings.TrimSpace(req.Email))
	displayName := strings.TrimSpace(req.DisplayName)
	if email == "" || displayName == "" {
		http.Error(w, "email and displayName required", http.StatusBadRequest)
		return
	}

	// Create user (StatusConflict / 409 on duplicate email)
	user, err := h.store.CreateUser(r.Context(), email, displayName)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			http.Error(w, "email already registered", http.StatusConflict)
			return
		}
		http.Error(w, "create user failed", http.StatusInternalServerError)
		return
	}

	// Build WebAuthn user adapter
	waUser := &webauthnUser{
		id:          user.ID.Bytes[:],
		email:       user.Email,
		displayName: user.DisplayName,
	}

	// Begin registration ceremony
	options, sessionData, err := h.webauthn.BeginRegistration(waUser)
	if err != nil {
		http.Error(w, "begin registration failed", http.StatusInternalServerError)
		return
	}

	// Serialize SessionData for persistence
	sessionJSON, err := json.Marshal(sessionData)
	if err != nil {
		http.Error(w, "marshal session failed", http.StatusInternalServerError)
		return
	}

	// Store session row
	session, err := h.store.CreateSession(r.Context(), user.ID.Bytes, sessionJSON, "register", 5*time.Minute)
	if err != nil {
		http.Error(w, "save session failed", http.StatusInternalServerError)
		return
	}

	// Set session cookie
	http.SetCookie(w, &http.Cookie{
		Name:     "lgv_webauthn_session",
		Value:    uuid.UUID(session.ID.Bytes).String(),
		Path:     "/api/v1/auth",
		HttpOnly: true,
		Secure:   false, // TODO: true in production
		SameSite: http.SameSiteStrictMode,
		MaxAge:   300,
	})

	// Return credential creation options
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(options)
}

func (h *Handler) RegisterVerify(w http.ResponseWriter, r *http.Request) {
	// Read session cookie
	cookie, err := r.Cookie("lgv_webauthn_session")
	if err != nil {
		http.Error(w, "missing session cookie", http.StatusUnauthorized)
		return
	}
	sessionID, err := uuid.Parse(cookie.Value)
	if err != nil {
		http.Error(w, "invalid session cookie", http.StatusUnauthorized)
		return
	}

	// Parse body
	var req registerVerifyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	if req.AgeRecipient == "" {
		http.Error(w, "ageRecipient required", http.StatusBadRequest)
		return
	}

	// Consume ceremony session row
	session, err := h.store.ConsumeSession(r.Context(), sessionID, "register")
	if err != nil {
		http.Error(w, "session expired or invalid", http.StatusUnauthorized)
		return
	}

	// Deserialize stored SessionData
	var sessionData webauthn.SessionData
	if err := json.Unmarshal(session.SessionData, &sessionData); err != nil {
		http.Error(w, "corrupt session", http.StatusInternalServerError)
		return
	}

	// Look up user
	user, err := h.store.GetUserByID(r.Context(), session.UserID)
	if err != nil {
		http.Error(w, "user not found", http.StatusUnauthorized)
		return
	}

	// Build WebAuthn user adapter (no credentials for registration)
	waUser := &webauthnUser{
		id:          user.ID.Bytes[:],
		email:       user.Email,
		displayName: user.DisplayName,
	}

	// Parse WebAuthn response from the raw JSON
	parsed, err := protocol.ParseCredentialCreationResponseBody(bytes.NewReader(req.Response))
	if err != nil {
		http.Error(w, "invalid registration response", http.StatusBadRequest)
		return
	}

	// Verify response against stored sessionData
	credential, err := h.webauthn.CreateCredential(waUser, sessionData, parsed)
	if err != nil {
		http.Error(w, "verification failed", http.StatusUnauthorized)
		return
	}

	// Save credential with its age recipient + optional nickname
	if _, err := h.store.CreateCredential(r.Context(), user.ID.Bytes, credential, req.AgeRecipient, req.Nickname); err != nil {
		http.Error(w, "save credential failed", http.StatusInternalServerError)
		return
	}

	// Issue a JWT for the new session
	token, err := h.jwt.Issue(user.ID.Bytes)
	if err != nil {
		http.Error(w, "issue token failed", http.StatusInternalServerError)
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:   "lgv_webauthn_session",
		Value:  "",
		Path:   "/api/v1/auth",
		MaxAge: -1,
	})
	h.jwt.SetSessionCookie(w, token)

	w.WriteHeader(http.StatusOK)
}
