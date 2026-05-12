package auth

import (
	"net/http"
	"time"

	"github.com/google/uuid"
)

const (
	SessionCookieName  = "lgv_session"
	ceremonyCookieName = "lgv_webauthn_session"
	ceremonyCookiePath = "/api/v1/auth"
	ceremonyTTL        = 5 * time.Minute
)

type Cookies struct {
	secure bool
	jwtTTL time.Duration
}

func NewCookies(secure bool, jwtTTL time.Duration) *Cookies {
	return &Cookies{secure: secure, jwtTTL: jwtTTL}
}

func (c *Cookies) SetSession(w http.ResponseWriter, token string) {
	http.SetCookie(w, &http.Cookie{
		Name:     SessionCookieName,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		Secure:   c.secure,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   int(c.jwtTTL.Seconds()),
	})
}

func (c *Cookies) ClearSession(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:   SessionCookieName,
		Value:  "",
		Path:   "/",
		MaxAge: -1,
	})
}

func (c *Cookies) SetCeremony(w http.ResponseWriter, sessionID uuid.UUID) {
	http.SetCookie(w, &http.Cookie{
		Name:     ceremonyCookieName,
		Value:    sessionID.String(),
		Path:     ceremonyCookiePath,
		HttpOnly: true,
		Secure:   c.secure,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   int(ceremonyTTL.Seconds()),
	})
}

func (c *Cookies) ClearCeremony(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:   ceremonyCookieName,
		Value:  "",
		Path:   ceremonyCookiePath,
		MaxAge: -1,
	})
}
