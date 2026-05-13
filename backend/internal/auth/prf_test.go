package auth

import (
	"crypto/sha256"
	"testing"
)

func TestAgeIdentitySaltStability(t *testing.T) {
	expected := sha256.Sum256([]byte("legavi.prf.age-identity.v1"))
	if ageIdentitySalt != expected {
		t.Fatal(`ageIdentitySalt drifted from sha256("legavi.prf.age-identity.v1"). This would lock every existing user out of their vault. Do not change.`)
	}
}
