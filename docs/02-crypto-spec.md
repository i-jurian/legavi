# 02 - Cryptographic Specification

No cryptographic primitives are implemented from scratch; everything composes existing libraries.

## Summary

Three pieces of cryptography:

- **WebAuthn passkeys** for sign-in. Face ID, Touch ID, Windows Hello, or a hardware security key. No passwords.
- **age encryption** (`age-encryption.org/v1`) for vault contents. Each entry is encrypted in the browser before upload, with the owner and any assigned contacts as recipients; any recipient can decrypt independently.
- **Ed25519 signatures** for the audit log. The owner signs periodic checkpoints over the log so tampering by the operator is detectable.

The WebAuthn PRF extension derives a stable encryption identity from each user's passkey. That identity is their age recipient. The server stores ciphertext and metadata only.

## 1. Trust premises

- The owner trusts their own device's hardware-backed keystore (Secure Enclave on Apple, TPM on Windows, FIDO2 on hardware keys).
- The operator is treated as zero-knowledge for owner data: ciphertext only, no plaintext access.
- Each contact independently trusts their own device's hardware-backed keystore.
- Network is adversarial; TLS 1.3 protects in transit but confidentiality guarantees hold at the application layer regardless.

## 2. Primitives in scope

| Primitive              | Purpose                                                            | Source                                                            |
| ---------------------- | ------------------------------------------------------------------ | ----------------------------------------------------------------- |
| WebAuthn passkey       | User authentication, hardware-bound credential                     | W3C WebAuthn Level 3                                              |
| WebAuthn PRF extension | Deterministic secret derivation from a passkey                     | W3C WebAuthn Extensions                                           |
| age                    | Multi-recipient encryption (X25519 + ChaCha20-Poly1305 internally) | `age-encryption.org/v1`                                           |
| Ed25519                | Signatures (audit log checkpoints)                                 | RFC 8032                                                          |
| BLAKE2b / SHA-256      | Hashing for audit log chain and domain separation                  | RFC 7693 / NIST FIPS 180-4                                        |
| Cryptographic random   | Salt, nonce, challenge generation                                  | OS RNG (`crypto/rand` on Go, `crypto.getRandomValues` on browser) |


## 3. Identity derivation from passkey

Each user (owner or contact) holds a WebAuthn passkey on their device. To derive their age encryption identity, the system invokes the WebAuthn PRF extension at authentication time with a fixed, well-known salt:

```
prf_salt      = sha256("legavi.prf.age-identity.v1")  // pinned by TestAgeIdentitySaltStability
prf_output    = WebAuthn PRF(passkey, prf_salt)
age_identity  = bech32("AGE-SECRET-KEY-", prf_output)
age_recipient = bech32("age", X25519_scalar_mult_base(prf_output))
```

`@noble/curves` and the age library clamp internally.

Properties:

- Deterministic: the same passkey produces the same age identity on every authentication.
- Cross-device consistent: a platform-synced passkey produces the same PRF output on every signed-in device.
- Hardware-bound: the passkey private material never leaves the authenticator. The PRF output is computed inside the authenticator and returned for use only after user verification (biometric or PIN).
- Domain-separated: distinct PRF inputs per use (see section 10).

The age private key exists in browser memory only for the duration of an active vault session. It is zeroed on lock (idle timeout, tab visibility change, or explicit logout).

## 4. Vault encryption

Every entry is age-encrypted to a recipient set that includes the owner plus zero or more assigned contacts:

```
ciphertext = age_encrypt(
  plaintext_entry,
  recipients = [owner_age_recipient, ...assigned_recipient_age_recipients]
)
```

The server stores the ciphertext along with metadata (entry ID, label hint, sort order, recipient assignment list).

Properties:

- **Owner can always decrypt their own data.** The owner's recipient is always included.
- **Each assigned recipient can independently decrypt** the entries assigned to them, after release fires (server-gated delivery).
- **No coordination required between recipients.** age multi-recipient is OR-of-recipients: any one of the listed recipients can decrypt independently.
- **Compartmentalization:** an entry is decryptable only by its listed recipients.
- **Server is never a recipient.** The operator cannot decrypt regardless of state.

### 4.1 Recipient assignment model

The owner assigns recipients per entry. Every entry includes the owner; contacts are optional:

| Assignment                            | Recipient list on the entry           | Enters release flow? |
| ------------------------------------- | ------------------------------------- | -------------------- |
| Owner only (private)                  | `[owner]`                             | No                   |
| Single recipient                      | `[owner, contact_alice]`              | Yes                  |
| Group of recipients (any can recover) | `[owner, contact_alice, contact_bob]` | Yes                  |

The owner can change recipient assignments at any time while alive. Re-assignment requires re-encrypting the affected entries with the new recipient set; this happens client-side in the owner's browser.

### 4.2 Reassignment semantics

When the owner changes an entry's recipient list:

1. Browser fetches the current ciphertext, decrypts with the owner's identity.
2. Browser re-encrypts to the new recipient set.
3. New ciphertext replaces the old on the server.
4. Audit log records the reassignment event with the entry ID and the (hashed) recipient list.

Because the previous ciphertext may have been observed by the previously-assigned recipient, reassignment cannot retroactively rescind access to data that was already disclosed.

## 5. Audit log

State-changing events (login, vault entry create/update/delete, contact add/remove, release state transitions) produce audit log entries. Each entry contains:

- Sequence number.
- Event type and payload.
- Timestamp (server clock).
- Hash of the previous entry.

Entries are hash-chained: any modification to entry `n` invalidates entry `n+1`'s hash, cascading forward.

Periodically (default: at every owner login, every release-state transition, and every 24 hours of activity), the owner signs a checkpoint over the current chain head with their Ed25519 audit key (derived from passkey PRF with domain-separated input). Checkpoints provide tamper-evidence against an operator who could otherwise rewrite the entire chain.

The owner's browser verifies the chain root against the latest signed checkpoint on each session. Mismatch raises an alert.

## 6. WebAuthn ceremony details

### 6.1 Registration

`@simplewebauthn/browser`'s `startRegistration` with `userVerification: required`, `residentKey: required`, and `extensions.prf.eval.first = <prf-salt>`. The server stores `credentialId` + COSE public key; the browser derives the age identity from the returned PRF output.

### 6.2 Authentication

`startAuthentication` with the same PRF salt. Server verifies the assertion against the stored public key; PRF output recomputes the same age identity deterministically. The PRF output never leaves the browser.

## 7. Contact onboarding

Owner submits email + display name; server emails an invitation link with a one-time token. Contact completes a WebAuthn registration ceremony on their own device; the browser submits the derived age recipient and credential. Owner verifies the recipient fingerprint out-of-band (phone or in person) and approves. The contact is then eligible to be assigned to vault entries; affected entries are re-encrypted client-side.

## 8. Backup export

The owner can export an encrypted backup of their vault. The export is a single age-encrypted file with two recipients:

- The owner's current age identity.
- An optional passphrase recipient (as strong as the passphrase chosen).

Export format:

```
age_encrypt(
  serialize(vault_entries, contact_metadata, audit_chain_head),
  recipients=[owner_age_recipient, optional(passphrase_recipient)]
)
```

The passphrase recipient uses age's built-in scrypt-based password support.

## 9. Revocation and rotation

- **Revoke a passkey:** owner removes the credentialId from the server's allowed list. Subsequent authentications with that passkey fail.
- **Revoke a contact:** owner marks the contact removed; their recipient is removed from any entry they were previously assigned to. The browser fetches each affected entry's ciphertext, decrypts with owner identity, re-encrypts to the reduced recipient set, uploads. If removing the contact leaves the entry with no contact recipients, the entry becomes owner-only and exits the release flow. The contact's credential record is retained for audit purposes but no new entries can be assigned to them.
- **Rotate the owner's passkey:** owner registers a new passkey, decrypts vault entries with the old identity, re-encrypts to the new identity plus the same recipients, and removes the old credential.

Rotation is owner-initiated and requires authentication with the current passkey.

## 10. Domain separation

PRF inputs and other derived material use distinct domain-separation tags to prevent cross-protocol confusion:

| Use                        | Tag (hashed with sha256 to derive PRF salt or used directly per row) |
| -------------------------- | -------------------------------------------------------------------- |
| Age identity               | `legavi.prf.age-identity.v1`                                         |
| Audit log signing          | `legavi.audit-signing.v1`                                            |
| Backup passphrase wrapping | `legavi.backup-passphrase.v1`                                        |
| Audit chain hash function  | `BLAKE2b("legavi.audit-chain.v1", entry_bytes)`                      |

Versioned suffixes allow future upgrades without ambiguity.

## 11. Wire format and schema versioning

Every encrypted blob includes a `schema_version` integer in its associated data (or in the surrounding wrapper for primitives that don't expose AAD). Breaking changes increment the version. Readers MUST reject unknown versions; no silent fallback.

Breaking changes require: bump of `schema_version` in affected blobs and tests covering both old and new versions until the deprecation window ends.

## 12. Implementation requirements

| Requirement                                                                                                                                                                                                                                 | Rationale                  |
| ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | -------------------------- |
| Use established libraries: `github.com/go-webauthn/webauthn` (Go); `@simplewebauthn/browser` (frontend); `@noble/curves` for X25519 scalar multiplication and Ed25519; `age-encryption` npm package for vault encryption. | No hand-rolled primitives. |
| Crypto code is colocated with its caller (`internal/auth/prf.go`, `frontend/src/lib/age-keypair.ts`); extracted into a dedicated `crypto/` package once the surface area justifies it. | Avoid premature isolation. |
| PRF salt is pinned by `TestAgeIdentitySaltStability` to `sha256("legavi.prf.age-identity.v1")`. Changing it locks every user out of their vault; rotate, do not edit. | Catch accidental drift. |
| Constant-time comparisons via `crypto/subtle` (Go) or `@noble/hashes/utils#equalBytes` (browser) for any equality check on secrets. | Side-channel resistance. |
| No `panic` in crypto code paths; errors only.                                                                                                                                                                                               | Predictable failure.       |
