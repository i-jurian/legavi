# 02 - Cryptographic Specification

**Update history**

- 2026-05-09: Initial draft

---

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

X25519, ChaCha20-Poly1305, and HKDF are used internally by `age` and WebAuthn but are never invoked directly by application code.

## 3. Identity derivation from passkey

Each user (owner or contact) holds a WebAuthn passkey on their device. To derive their age encryption identity, the system invokes the WebAuthn PRF extension at authentication time with a fixed, well-known salt:

```
prf_input = "legavi.age-identity.v1"
prf_output = WebAuthn PRF(passkey, prf_input)  // 32 bytes
age_private_key = X25519_clamp(prf_output)
age_recipient = X25519_public(age_private_key)
```

Properties:

- Deterministic: the same passkey produces the same age identity on every authentication.
- Cross-device consistent: a platform-synced passkey produces the same PRF output on every signed-in device.
- Hardware-bound: the passkey private material never leaves the authenticator. The PRF output is computed inside the authenticator and returned for use only after user verification (biometric or PIN).
- Domain-separated: a separate PRF input (`legavi.audit-signing.v1`) derives the audit log signing key independently from the encryption key.

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

```
1. Server generates a random challenge.
2. Browser invokes navigator.credentials.create({
     challenge,
     rp: { name: "Legavi", id: "<host>" },
     user: { id, name, displayName },
     pubKeyCredParams: [{ alg: -7, type: "public-key" }, { alg: -8, type: "public-key" }],
     authenticatorSelection: {
       userVerification: "required",
       residentKey: "required"
     },
     extensions: { prf: { eval: { first: <prf-input-bytes> } } }
   })
3. User completes biometric or PIN ceremony in their authenticator.
4. Browser receives the AttestationResponse, extracts:
   - credentialId
   - public key (COSE format)
   - PRF output (already computed during registration)
5. Browser derives age identity from PRF output, retains it for the session.
6. Browser sends to server: credentialId, public key, attestation statement.
7. Server verifies attestation, stores credentialId + public key for the user.
```

### 6.2 Authentication

```
1. Server generates a random challenge.
2. Browser invokes navigator.credentials.get({
     challenge,
     allowCredentials: [...],
     userVerification: "required",
     extensions: { prf: { eval: { first: <prf-input-bytes> } } }
   })
3. User completes biometric or PIN ceremony.
4. Browser receives the AssertionResponse with PRF output.
5. Browser derives age identity from PRF output for this session.
6. Browser sends to server: credentialId, signature over challenge.
7. Server verifies signature against the stored public key.
8. Session established.
```

The PRF output never leaves the browser; the server sees only the public credential.

## 7. Contact onboarding

```
1. Owner enters contact's email and display name.
2. Server emails the contact an invitation link with a one-time token.
3. Contact clicks link, lands on registration page.
4. Contact completes WebAuthn registration ceremony (their device, their passkey).
5. Browser computes contact's age recipient from their PRF output.
6. Contact's age recipient and credential are sent to the server.
7. Owner verifies the contact's identity out-of-band by reading the contact's age recipient fingerprint to them over a known-good channel (phone, in person).
8. Owner approves the contact, server records the verified-recipient binding.
9. The contact is now eligible to be assigned to vault entries. The owner can then assign (or reassign) entries to this contact via the vault UI; each affected entry is re-encrypted client-side to the new recipient set.
```

## 8. Backup export

The owner can export an encrypted backup of their vault. The export is a single age-encrypted file with two recipients:

- The owner's current age identity (so they can decrypt with their current passkey).
- An optional passphrase recipient (so they can decrypt with a written-down passphrase even if their passkey is lost). A passphrase backup is only as strong as the passphrase; a weak one can be guessed by anyone who obtains the export file.

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

| Use                        | Tag                                             |
| -------------------------- | ----------------------------------------------- |
| Age identity               | `legavi.age-identity.v1`                        |
| Audit log signing          | `legavi.audit-signing.v1`                       |
| Backup passphrase wrapping | `legavi.backup-passphrase.v1`                   |
| Audit chain hash function  | `BLAKE2b("legavi.audit-chain.v1", entry_bytes)` |

Versioned suffixes allow future upgrades without ambiguity.

## 11. Wire format and schema versioning

Every encrypted blob includes a `schema_version` integer in its associated data (or in the surrounding wrapper for primitives that don't expose AAD). Breaking changes increment the version. Readers MUST reject unknown versions; no silent fallback.

Breaking changes require: bump of `schema_version` in affected blobs, a migration plan in [Architecture section 10](03-architecture.md), and tests covering both old and new versions until the deprecation window ends.

## 12. Implementation requirements

| Requirement                                                                                                                                                                                               | Rationale                  |
| --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | -------------------------- |
| Use established libraries: `filippo.io/age` (Go); `age-encryption` npm package (browser); `@simplewebauthn/server` and `@simplewebauthn/browser` for WebAuthn; `@noble/curves/ed25519` for audit signing. | No hand-rolled primitives. |
| All cryptographic code in a separate `crypto/` package with no business logic dependencies.                                                                                                               | Auditability.              |
| Test vectors from RFCs and library documentation checked into `crypto/testdata/`.                                                                                                                         | Catch regressions.         |
| Constant-time comparisons via `crypto/subtle` (Go) or `@noble/hashes/utils#equalBytes` (browser) for any equality check on secrets.                                                                       | Side-channel resistance.   |
| No `panic` in crypto code paths; errors only.                                                                                                                                                             | Predictable failure.       |
