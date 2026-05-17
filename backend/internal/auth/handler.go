package auth

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/webauthn"
	"github.com/google/uuid"

	"github.com/i-jurian/legavi/backend/internal/respond"
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
	Nickname     string          `json:"nickname"`
	Response     json.RawMessage `json:"response"`
}

type loginStartRequest struct {
	Email string `json:"email"`
}

type loginVerifyRequest struct {
	Response json.RawMessage `json:"response"`
}

type meResponse struct {
	ID          string `json:"id"`
	Email       string `json:"email"`
	DisplayName string `json:"displayName"`
}

func NewHandler(wa *WebAuthn, j *JWT, s *store.Store, c *Cookies) *Handler {
	return &Handler{webauthn: wa, jwt: j, store: s, cookies: c}
}

func (u *webauthnUser) WebAuthnID() []byte                         { return u.id }
func (u *webauthnUser) WebAuthnName() string                       { return u.email }
func (u *webauthnUser) WebAuthnDisplayName() string                { return u.displayName }
func (u *webauthnUser) WebAuthnCredentials() []webauthn.Credential { return u.credentials }

func (h *Handler) RegisterStart(w http.ResponseWriter, r *http.Request) *respond.Error {
	// Validate request payload
	var req registerStartRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return respond.BadRequest("invalid json", err)
	}
	email := strings.ToLower(strings.TrimSpace(req.Email))
	displayName := strings.TrimSpace(req.DisplayName)
	if email == "" || displayName == "" {
		return respond.BadRequest("email and displayName required")
	}

	user, err := h.store.GetUserByEmail(r.Context(), email)
	if err == nil {
		count, err := h.store.CountUserCredentials(r.Context(), user.ID)
		if err != nil {
			return respond.Internal(fmt.Errorf("lookup credentials: %w", err))
		}

		// 409 only if a credential is already attached; otherwise reuse the orphan row
		if count > 0 {
			return respond.Conflict("email already registered")
		}
	} else {
		user, err = h.store.CreateUser(r.Context(), email, displayName)
		if err != nil {
			return respond.Internal(fmt.Errorf("create user: %w", err))
		}
	}

	waUser := &webauthnUser{
		id:          user.ID.Bytes[:],
		email:       user.Email,
		displayName: user.DisplayName,
	}

	// Begin registration ceremony
	options, sessionData, err := h.webauthn.BeginRegistration(waUser, webauthn.WithExtensions(ageIdentityExtensions()))
	if err != nil {
		return respond.Internal(fmt.Errorf("begin registration: %w", err))
	}

	// Persist challenge state for the verify step
	sessionJSON, err := json.Marshal(sessionData)
	if err != nil {
		return respond.Internal(fmt.Errorf("marshal session: %w", err))
	}
	session, err := h.store.CreateSession(r.Context(), user.ID.Bytes, sessionJSON, "register", ceremonyTTL)
	if err != nil {
		return respond.Internal(fmt.Errorf("save session: %w", err))
	}

	// Bind ceremony to this browser via cookie
	h.cookies.SetCeremony(w, uuid.UUID(session.ID.Bytes))

	// Send challenge to the browser
	respond.JSON(w, http.StatusOK, options)
	return nil
}

func (h *Handler) RegisterVerify(w http.ResponseWriter, r *http.Request) *respond.Error {
	// Validate session cookie
	cookie, err := r.Cookie(ceremonyCookieName)
	if err != nil {
		return respond.Unauthorized("missing session cookie")
	}
	sessionID, err := uuid.Parse(cookie.Value)
	if err != nil {
		return respond.Unauthorized("invalid session cookie")
	}

	// Validate request payload
	var req registerVerifyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return respond.BadRequest("invalid json", err)
	}
	nickname := strings.TrimSpace(req.Nickname)
	if req.AgeRecipient == "" || nickname == "" {
		return respond.BadRequest("ageRecipient and nickname required")
	}

	// Bind verify to its start ceremony (single-use)
	session, err := h.store.ConsumeSession(r.Context(), sessionID, "register")
	if err != nil {
		return respond.Unauthorized("session expired or invalid")
	}

	// Restore challenge state to verify
	var sessionData webauthn.SessionData
	if err := json.Unmarshal(session.SessionData, &sessionData); err != nil {
		return respond.Internal(fmt.Errorf("corrupt session: %w", err))
	}

	// Validate user exists
	user, err := h.store.GetUserByID(r.Context(), session.UserID)
	if err != nil {
		return respond.Unauthorized("user not found")
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
		return respond.BadRequest("invalid registration response", err)
	}

	// Verify attestation against the original challenge
	credential, err := h.webauthn.CreateCredential(waUser, sessionData, parsed)
	if err != nil {
		return respond.Unauthorized("verification failed", err)
	}

	// Persist new credential for this user
	if _, err := h.store.CreateCredential(r.Context(), user.ID.Bytes, credential, req.AgeRecipient, nickname); err != nil {
		return respond.Internal(fmt.Errorf("save credential: %w", err))
	}

	// Authenticate the user for subsequent requests
	token, err := h.jwt.Issue(user.ID.Bytes)
	if err != nil {
		return respond.Internal(fmt.Errorf("issue token: %w", err))
	}

	h.cookies.ClearCeremony(w)
	h.cookies.SetSession(w, token)

	w.WriteHeader(http.StatusOK)
	return nil
}

func (h *Handler) LoginStart(w http.ResponseWriter, r *http.Request) *respond.Error {
	// Validate request payload
	var req loginStartRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return respond.BadRequest("invalid json", err)
	}
	email := strings.ToLower(strings.TrimSpace(req.Email))
	if email == "" {
		return respond.BadRequest("email required")
	}

	// Validate user exists (vague 401 to avoid leaking which emails exist)
	user, err := h.store.GetUserByEmail(r.Context(), email)
	if err != nil {
		return respond.Unauthorized("invalid credentials")
	}

	// Build credential allowlist for this user
	creds, err := h.store.ListUserWebAuthnCredentials(r.Context(), user.ID.Bytes)
	if err != nil {
		return respond.Internal(fmt.Errorf("lookup credentials: %w", err))
	}
	if len(creds) == 0 {
		return respond.Unauthorized("invalid credentials")
	}

	waUser := &webauthnUser{
		id:          user.ID.Bytes[:],
		email:       user.Email,
		displayName: user.DisplayName,
		credentials: creds,
	}

	// Begin login ceremony
	options, sessionData, err := h.webauthn.BeginLogin(waUser, webauthn.WithAssertionExtensions(ageIdentityExtensions()))
	if err != nil {
		return respond.Internal(fmt.Errorf("begin login: %w", err))
	}

	// Persist challenge state for the verify step
	sessionJSON, err := json.Marshal(sessionData)
	if err != nil {
		return respond.Internal(fmt.Errorf("marshal session: %w", err))
	}
	session, err := h.store.CreateSession(r.Context(), user.ID.Bytes, sessionJSON, "login", ceremonyTTL)
	if err != nil {
		return respond.Internal(fmt.Errorf("save session: %w", err))
	}

	// Bind ceremony to this browser via cookie
	h.cookies.SetCeremony(w, uuid.UUID(session.ID.Bytes))

	// Send challenge to the browser
	respond.JSON(w, http.StatusOK, options)
	return nil
}

func (h *Handler) LoginVerify(w http.ResponseWriter, r *http.Request) *respond.Error {
	// Validate session cookie
	cookie, err := r.Cookie(ceremonyCookieName)
	if err != nil {
		return respond.Unauthorized("missing session cookie")
	}
	sessionID, err := uuid.Parse(cookie.Value)
	if err != nil {
		return respond.Unauthorized("invalid session cookie")
	}

	// Validate request payload
	var req loginVerifyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return respond.BadRequest("invalid json", err)
	}

	// Bind verify to its start ceremony (single-use)
	session, err := h.store.ConsumeSession(r.Context(), sessionID, "login")
	if err != nil {
		return respond.Unauthorized("session expired or invalid")
	}

	// Restore challenge state to verify
	var sessionData webauthn.SessionData
	if err := json.Unmarshal(session.SessionData, &sessionData); err != nil {
		return respond.Internal(fmt.Errorf("corrupt session: %w", err))
	}

	// Validate user exists
	user, err := h.store.GetUserByID(r.Context(), session.UserID)
	if err != nil {
		return respond.Unauthorized("user not found")
	}

	// Build credential allowlist for matching the assertion
	creds, err := h.store.ListUserWebAuthnCredentials(r.Context(), user.ID.Bytes)
	if err != nil {
		return respond.Internal(fmt.Errorf("lookup credentials: %w", err))
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
		return respond.BadRequest("invalid assertion", err)
	}

	// Verify assertion against the original challenge
	credential, err := h.webauthn.ValidateLogin(waUser, sessionData, parsed)
	if err != nil {
		return respond.Unauthorized("verification failed", err)
	}

	// Record credential usage (cloning detection + audit)
	if err := h.store.UpdateCredentialUsage(r.Context(), credential.ID, credential.Authenticator.SignCount); err != nil {
		return respond.Internal(fmt.Errorf("update credential: %w", err))
	}

	// Authenticate the user for subsequent requests
	token, err := h.jwt.Issue(user.ID.Bytes)
	if err != nil {
		return respond.Internal(fmt.Errorf("issue token: %w", err))
	}

	h.cookies.ClearCeremony(w)
	h.cookies.SetSession(w, token)

	w.WriteHeader(http.StatusOK)
	return nil
}

func (h *Handler) UnlockStart(w http.ResponseWriter, r *http.Request) *respond.Error {
	userID, ok := UserIDFromContext(r.Context())
	if !ok {
		return respond.Unauthorized("unauthorized")
	}

	// Validate user exists
	user, err := h.store.GetUser(r.Context(), userID)
	if err != nil {
		return respond.Unauthorized("user not found")
	}

	// Build credential allowlist for this user
	creds, err := h.store.ListUserWebAuthnCredentials(r.Context(), user.ID.Bytes)
	if err != nil {
		return respond.Internal(fmt.Errorf("lookup credentials: %w", err))
	}
	if len(creds) == 0 {
		return respond.Unauthorized("no credentials")
	}

	waUser := &webauthnUser{
		id:          user.ID.Bytes[:],
		email:       user.Email,
		displayName: user.DisplayName,
		credentials: creds,
	}

	// Begin unlock ceremony (PRF re-derivation only; no new session cookie issued)
	options, sessionData, err := h.webauthn.BeginLogin(waUser, webauthn.WithAssertionExtensions(ageIdentityExtensions()))
	if err != nil {
		return respond.Internal(fmt.Errorf("begin login: %w", err))
	}

	// Persist challenge state for the verify step
	sessionJSON, err := json.Marshal(sessionData)
	if err != nil {
		return respond.Internal(fmt.Errorf("marshal session: %w", err))
	}
	session, err := h.store.CreateSession(r.Context(), user.ID.Bytes, sessionJSON, "login", ceremonyTTL)
	if err != nil {
		return respond.Internal(fmt.Errorf("save session: %w", err))
	}

	// Bind ceremony to this browser via cookie
	h.cookies.SetCeremony(w, uuid.UUID(session.ID.Bytes))

	// Send challenge to the browser
	respond.JSON(w, http.StatusOK, options)
	return nil
}

func (h *Handler) UnlockVerify(w http.ResponseWriter, r *http.Request) *respond.Error {
	userID, ok := UserIDFromContext(r.Context())
	if !ok {
		return respond.Unauthorized("unauthorized")
	}

	// Validate ceremony cookie
	cookie, err := r.Cookie(ceremonyCookieName)
	if err != nil {
		return respond.Unauthorized("missing session cookie")
	}
	sessionID, err := uuid.Parse(cookie.Value)
	if err != nil {
		return respond.Unauthorized("invalid session cookie")
	}

	// Validate request payload
	var req loginVerifyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return respond.BadRequest("invalid json", err)
	}

	// Bind verify to its start ceremony (single-use)
	session, err := h.store.ConsumeSession(r.Context(), sessionID, "login")
	if err != nil {
		return respond.Unauthorized("session expired or invalid")
	}

	// Ensure the ceremony session belongs to the authenticated user
	if uuid.UUID(session.UserID.Bytes) != userID {
		return respond.Unauthorized("session mismatch")
	}

	// Restore challenge state to verify
	var sessionData webauthn.SessionData
	if err := json.Unmarshal(session.SessionData, &sessionData); err != nil {
		return respond.Internal(fmt.Errorf("corrupt session: %w", err))
	}

	// Validate user exists
	user, err := h.store.GetUserByID(r.Context(), session.UserID)
	if err != nil {
		return respond.Unauthorized("user not found")
	}

	// Build credential allowlist for matching the assertion
	creds, err := h.store.ListUserWebAuthnCredentials(r.Context(), user.ID.Bytes)
	if err != nil {
		return respond.Internal(fmt.Errorf("lookup credentials: %w", err))
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
		return respond.BadRequest("invalid assertion", err)
	}

	// Verify assertion against the original challenge
	credential, err := h.webauthn.ValidateLogin(waUser, sessionData, parsed)
	if err != nil {
		return respond.Unauthorized("verification failed", err)
	}

	// Record credential usage (cloning detection + audit)
	if err := h.store.UpdateCredentialUsage(r.Context(), credential.ID, credential.Authenticator.SignCount); err != nil {
		return respond.Internal(fmt.Errorf("update credential: %w", err))
	}

	h.cookies.ClearCeremony(w)

	w.WriteHeader(http.StatusNoContent)
	return nil
}

func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) *respond.Error {
	h.cookies.ClearSession(w)
	w.WriteHeader(http.StatusNoContent)
	return nil
}

func (h *Handler) Me(w http.ResponseWriter, r *http.Request) *respond.Error {
	userID, ok := UserIDFromContext(r.Context())
	if !ok {
		return respond.Unauthorized("unauthorized")
	}
	user, err := h.store.GetUser(r.Context(), userID)
	if err != nil {
		return respond.Unauthorized("user not found")
	}
	respond.JSON(w, http.StatusOK, meResponse{
		ID:          userID.String(),
		Email:       user.Email,
		DisplayName: user.DisplayName,
	})
	return nil
}
