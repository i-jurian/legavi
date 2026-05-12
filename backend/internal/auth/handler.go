package auth

import (
	"bytes"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/webauthn"
	"github.com/google/uuid"

	"github.com/i-jurian/legavi/backend/internal/store"
)

type Handler struct {
	webauthn *WebAuthn
	jwt      *JWT
	store    *store.Store
	cookies  *Cookies
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

type loginStartRequest struct {
	Email string `json:"email"`
}

type loginVerifyRequest struct {
	Response json.RawMessage `json:"response"`
}

func NewHandler(wa *WebAuthn, j *JWT, s *store.Store, c *Cookies) *Handler {
	return &Handler{webauthn: wa, jwt: j, store: s, cookies: c}
}

func (u *webauthnUser) WebAuthnID() []byte                         { return u.id }
func (u *webauthnUser) WebAuthnName() string                       { return u.email }
func (u *webauthnUser) WebAuthnDisplayName() string                { return u.displayName }
func (u *webauthnUser) WebAuthnCredentials() []webauthn.Credential { return u.credentials }

func (h *Handler) RegisterStart(w http.ResponseWriter, r *http.Request) {
	// Validate request payload
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

	user, err := h.store.GetUserByEmail(r.Context(), email)
	if err == nil {
		count, err := h.store.CountUserCredentials(r.Context(), user.ID)
		if err != nil {
			http.Error(w, "lookup credentials failed", http.StatusInternalServerError)
			return
		}

		// 409 only if a credential is already attached; otherwise reuse the orphan row
		if count > 0 {
			http.Error(w, "email already registered", http.StatusConflict)
			return
		}
	} else {
		user, err = h.store.CreateUser(r.Context(), email, displayName)
		if err != nil {
			http.Error(w, "create user failed", http.StatusInternalServerError)
			return
		}
	}

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

	// Persist challenge state for the verify step
	sessionJSON, err := json.Marshal(sessionData)
	if err != nil {
		http.Error(w, "marshal session failed", http.StatusInternalServerError)
		return
	}
	session, err := h.store.CreateSession(r.Context(), user.ID.Bytes, sessionJSON, "register", ceremonyTTL)
	if err != nil {
		http.Error(w, "save session failed", http.StatusInternalServerError)
		return
	}

	// Bind ceremony to this browser via cookie
	h.cookies.SetCeremony(w, uuid.UUID(session.ID.Bytes))

	// Send challenge to the browser
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(options)
}

func (h *Handler) RegisterVerify(w http.ResponseWriter, r *http.Request) {
	// Validate session cookie
	cookie, err := r.Cookie(ceremonyCookieName)
	if err != nil {
		http.Error(w, "missing session cookie", http.StatusUnauthorized)
		return
	}
	sessionID, err := uuid.Parse(cookie.Value)
	if err != nil {
		http.Error(w, "invalid session cookie", http.StatusUnauthorized)
		return
	}

	// Validate request payload
	var req registerVerifyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	if req.AgeRecipient == "" {
		http.Error(w, "ageRecipient required", http.StatusBadRequest)
		return
	}

	// Bind verify to its start ceremony (single-use)
	session, err := h.store.ConsumeSession(r.Context(), sessionID, "register")
	if err != nil {
		http.Error(w, "session expired or invalid", http.StatusUnauthorized)
		return
	}

	// Restore challenge state to verify
	var sessionData webauthn.SessionData
	if err := json.Unmarshal(session.SessionData, &sessionData); err != nil {
		http.Error(w, "corrupt session", http.StatusInternalServerError)
		return
	}

	// Validate user exists
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

	// Decode browser attestation
	parsed, err := protocol.ParseCredentialCreationResponseBody(bytes.NewReader(req.Response))
	if err != nil {
		http.Error(w, "invalid registration response", http.StatusBadRequest)
		return
	}

	// Verify attestation against the original challenge
	credential, err := h.webauthn.CreateCredential(waUser, sessionData, parsed)
	if err != nil {
		http.Error(w, "verification failed", http.StatusUnauthorized)
		return
	}

	// Persist new credential for this user
	if _, err := h.store.CreateCredential(r.Context(), user.ID.Bytes, credential, req.AgeRecipient, req.Nickname); err != nil {
		http.Error(w, "save credential failed", http.StatusInternalServerError)
		return
	}

	// Authenticate the user for subsequent requests
	token, err := h.jwt.Issue(user.ID.Bytes)
	if err != nil {
		http.Error(w, "issue token failed", http.StatusInternalServerError)
		return
	}

	h.cookies.ClearCeremony(w)
	h.cookies.SetSession(w, token)

	w.WriteHeader(http.StatusOK)
}

func (h *Handler) LoginStart(w http.ResponseWriter, r *http.Request) {
	// Validate request payload
	var req loginStartRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	email := strings.ToLower(strings.TrimSpace(req.Email))
	if email == "" {
		http.Error(w, "email required", http.StatusBadRequest)
		return
	}

	// Validate user exists (vague 401 to avoid leaking which emails exist)
	user, err := h.store.GetUserByEmail(r.Context(), email)
	if err != nil {
		http.Error(w, "invalid credentials", http.StatusUnauthorized)
		return
	}

	// Build credential allowlist for this user
	creds, err := h.store.ListUserWebAuthnCredentials(r.Context(), user.ID.Bytes)
	if err != nil {
		http.Error(w, "lookup credentials failed", http.StatusInternalServerError)
		return
	}
	if len(creds) == 0 {
		http.Error(w, "invalid credentials", http.StatusUnauthorized)
		return
	}

	waUser := &webauthnUser{
		id:          user.ID.Bytes[:],
		email:       user.Email,
		displayName: user.DisplayName,
		credentials: creds,
	}

	// Begin login ceremony
	options, sessionData, err := h.webauthn.BeginLogin(waUser)
	if err != nil {
		http.Error(w, "begin login failed", http.StatusInternalServerError)
		return
	}

	// Persist challenge state for the verify step
	sessionJSON, err := json.Marshal(sessionData)
	if err != nil {
		http.Error(w, "marshal session failed", http.StatusInternalServerError)
		return
	}
	session, err := h.store.CreateSession(r.Context(), user.ID.Bytes, sessionJSON, "login", ceremonyTTL)
	if err != nil {
		http.Error(w, "save session failed", http.StatusInternalServerError)
		return
	}

	// Bind ceremony to this browser via cookie
	h.cookies.SetCeremony(w, uuid.UUID(session.ID.Bytes))

	// Send challenge to the browser
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(options)
}

func (h *Handler) LoginVerify(w http.ResponseWriter, r *http.Request) {
	// Validate session cookie
	cookie, err := r.Cookie(ceremonyCookieName)
	if err != nil {
		http.Error(w, "missing session cookie", http.StatusUnauthorized)
		return
	}
	sessionID, err := uuid.Parse(cookie.Value)
	if err != nil {
		http.Error(w, "invalid session cookie", http.StatusUnauthorized)
		return
	}

	// Validate request payload
	var req loginVerifyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}

	// Bind verify to its start ceremony (single-use)
	session, err := h.store.ConsumeSession(r.Context(), sessionID, "login")
	if err != nil {
		http.Error(w, "session expired or invalid", http.StatusUnauthorized)
		return
	}

	// Restore challenge state to verify
	var sessionData webauthn.SessionData
	if err := json.Unmarshal(session.SessionData, &sessionData); err != nil {
		http.Error(w, "corrupt session", http.StatusInternalServerError)
		return
	}

	// Validate user exists
	user, err := h.store.GetUserByID(r.Context(), session.UserID)
	if err != nil {
		http.Error(w, "user not found", http.StatusUnauthorized)
		return
	}

	// Build credential allowlist for matching the assertion
	creds, err := h.store.ListUserWebAuthnCredentials(r.Context(), user.ID.Bytes)
	if err != nil {
		http.Error(w, "lookup credentials failed", http.StatusInternalServerError)
		return
	}

	waUser := &webauthnUser{
		id:          user.ID.Bytes[:],
		email:       user.Email,
		displayName: user.DisplayName,
		credentials: creds,
	}

	// Decode browser assertion
	parsed, err := protocol.ParseCredentialRequestResponseBody(bytes.NewReader(req.Response))
	if err != nil {
		http.Error(w, "invalid assertion", http.StatusBadRequest)
		return
	}

	// Verify assertion against the original challenge
	credential, err := h.webauthn.ValidateLogin(waUser, sessionData, parsed)
	if err != nil {
		http.Error(w, "verification failed", http.StatusUnauthorized)
		return
	}

	// Record credential usage (cloning detection + audit)
	if err := h.store.UpdateCredentialUsage(r.Context(), credential.ID, credential.Authenticator.SignCount); err != nil {
		http.Error(w, "update credential failed", http.StatusInternalServerError)
		return
	}

	// Authenticate the user for subsequent requests
	token, err := h.jwt.Issue(user.ID.Bytes)
	if err != nil {
		http.Error(w, "issue token failed", http.StatusInternalServerError)
		return
	}

	h.cookies.ClearCeremony(w)
	h.cookies.SetSession(w, token)

	w.WriteHeader(http.StatusOK)
}

func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) {
	h.cookies.ClearSession(w)
	w.WriteHeader(http.StatusNoContent)
}
