// Package auth provides WebAuthn registration and login, plus JWT session management.
package auth

import (
	"fmt"
	"net/url"

	"github.com/go-webauthn/webauthn/webauthn"
)

type WebAuthn struct {
	*webauthn.WebAuthn
}

func NewWebAuthn(publicURL string) (*WebAuthn, error) {
	parsed, err := url.Parse(publicURL)
	if err != nil {
		return nil, fmt.Errorf("invalid public url: %w", err)
	}

	w, err := webauthn.New(&webauthn.Config{
		RPID:          parsed.Hostname(),
		RPDisplayName: "Legavi",
		RPOrigins:     []string{publicURL},
	})
	if err != nil {
		return nil, fmt.Errorf("create webauthn: %w", err)
	}

	return &WebAuthn{w}, nil
}
