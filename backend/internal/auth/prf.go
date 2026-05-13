package auth

import "github.com/go-webauthn/webauthn/protocol"

// ageIdentitySalt MUST NEVER CHANGE; changing it locks all users out of their vault forever.
// Value is sha256("legavi.prf.age-identity.v1"); pinned by TestAgeIdentitySaltStability.
var ageIdentitySalt = [32]byte{
	0xaf, 0xcb, 0xd7, 0xe7, 0xdb, 0xfe, 0xe8, 0xea,
	0xa6, 0xee, 0x80, 0xd7, 0x8b, 0x42, 0xf1, 0xc5,
	0x25, 0xc4, 0xe2, 0xec, 0x57, 0xb7, 0x26, 0x7d,
	0x9a, 0x24, 0x90, 0xd3, 0x24, 0xc0, 0x02, 0x16,
}

func ageIdentityExtensions() protocol.AuthenticationExtensions {
	return protocol.AuthenticationExtensions{
		"prf": map[string]any{
			"eval": map[string]any{
				"first": ageIdentitySalt[:],
			},
		},
	}
}
