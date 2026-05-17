# 05 - API Specification

HTTP API. The handlers in [backend/internal/auth](../backend/internal/auth) are the source of truth; no OpenAPI document yet.

## 1. Conventions

- **Base path:** `/api/v1/...`.
- **Transport:** JSON over HTTPS (TLS 1.3 in production). Dev runs over HTTP via Vite's proxy.
- **Authentication:** session JWT in `lgv_session` (httponly, `SameSite=Strict`, `Path=/`, `Secure` over HTTPS, `Max-Age=LGV_JWT_TTL`). No `Authorization` header or refresh token. Browsers send `credentials: "include"`. Ceremony cookie `lgv_webauthn_session` (`/api/v1/auth`, 5 min) bridges `*/start` -> `*/verify`.
- **CSRF:** `SameSite=Strict` on `lgv_session` is the mitigation. No double-submit token.
- **Idempotency:** no `Idempotency-Key` header. Worker-side dedup via `jobs.dedup_key`; request-path duplicates fall to unique constraints.
- **Errors:** plain text body + HTTP status. No JSON envelope.
- **Rate limits:** one per-IP bucket over `/api/*`; `/healthz` and `/readyz` are outside it (section 11).
- **Content type:** `application/json`; field naming is `camelCase`.

## 2. Authentication endpoints

Ceremony endpoints are anonymous; `logout` and `me` require `lgv_session`. The `response` field on verify endpoints is the full JSON from `@simplewebauthn/browser`.

### 2.1 `POST /api/v1/auth/register/start`

Begin WebAuthn registration. New email creates a `users` row; orphan row (email, zero credentials) is reused; 409 if the email already has a credential.

**Request:**

```json
{
  "email": "user@example.com",
  "displayName": "User Name"
}
```

**Response 200:** go-webauthn `CredentialCreation`, wrapped under `publicKey`:

```json
{
  "publicKey": {
    "rp": { "name": "Legavi", "id": "localhost" },
    "user": {
      "id": "base64url-...",
      "name": "user@example.com",
      "displayName": "User Name"
    },
    "challenge": "base64url-...",
    "pubKeyCredParams": [
      { "alg": -7, "type": "public-key" },
      { "alg": -8, "type": "public-key" }
    ],
    "authenticatorSelection": {
      "userVerification": "required",
      "residentKey": "required"
    },
    "extensions": { "prf": { "eval": { "first": "base64url-..." } } }
  }
}
```

Also sets the `lgv_webauthn_session` ceremony cookie.

**Errors:**

- `400` empty `email` or `displayName`.
- `409` email already has a credential.

### 2.2 `POST /api/v1/auth/register/verify`

Submit attestation. Requires the ceremony cookie.

**Request:**

```json
{
  "ageRecipient": "age1...",
  "nickname": "MacBook",
  "response": { "id": "...", "rawId": "...", "response": { "...": "RegistrationResponseJSON from @simplewebauthn/browser" }, "clientExtensionResults": { "prf": { "results": { "first": "..." } } }, "type": "public-key" }
}
```

`ageRecipient` is the bech32 `age1...` derived in the browser from the PRF output. `nickname` is a device label shown in settings.

**Response 200:** empty body. Sets `lgv_session`; clears the ceremony cookie.

**Errors:**

- `400` malformed JSON or attestation; empty `ageRecipient` or `nickname`.
- `401` missing/expired ceremony cookie or attestation verification failure.

### 2.3 `POST /api/v1/auth/login/start`

**Request:**

```json
{ "email": "user@example.com" }
```

**Response 200:** go-webauthn `CredentialAssertion` wrapped under `publicKey`, with `allowCredentials` for the user and the same PRF salt as registration:

```json
{
  "publicKey": {
    "challenge": "base64url-...",
    "allowCredentials": [
      { "type": "public-key", "id": "base64url-...", "transports": ["internal"] }
    ],
    "userVerification": "required",
    "extensions": { "prf": { "eval": { "first": "base64url-..." } } }
  }
}
```

Also sets the ceremony cookie.

**Errors:**

- `400` empty email.
- `401` unknown email or no credentials. Email existence is enumerable here (see [Threat Model section 4](01-threat-model.md)).

### 2.4 `POST /api/v1/auth/login/verify`

**Request:**

```json
{
  "response": { "id": "...", "rawId": "...", "response": { "...": "AuthenticationResponseJSON from @simplewebauthn/browser" }, "clientExtensionResults": { "prf": { "results": { "first": "..." } } }, "type": "public-key" }
}
```

**Response 200:** empty body. Sets `lgv_session`; clears the ceremony cookie. Updates `sign_count` and `last_used_at` on the credential.

**Errors:**

- `400` malformed JSON or assertion.
- `401` missing ceremony cookie or assertion verification failure.

### 2.5 `POST /api/v1/auth/logout`

Authenticated. Clears `lgv_session`. **Response 204.** Anonymous request returns `401`.

### 2.6 `GET /api/v1/auth/me`

Authenticated. Used by the frontend route guard.

**Response 200:**

```json
{
  "id": "uuid-...",
  "email": "user@example.com",
  "displayName": "User Name"
}
```

**Errors:** `401` if the cookie is missing/expired or the user no longer exists.

### 2.7 `POST /api/v1/auth/unlock/start`

Authenticated. Begins a WebAuthn assertion for the current user without re-typing email, so the browser can re-derive the PRF output.

**Request:** empty body.

**Response 200:** same shape as `login/start`. Also sets the ceremony cookie.

**Errors:** `401` if the session is missing, the user is not found, or no credentials exist.

### 2.8 `POST /api/v1/auth/unlock/verify`

Authenticated. Verifies the assertion, clears the ceremony cookie. Session cookie is not re-issued.

**Request:** same shape as `login/verify`.

**Response 204:** no body.

**Errors:**

- `400` malformed JSON or assertion.
- `401` missing ceremony cookie, session mismatch with the ceremony, or assertion verification failure.

## 3. Check-in endpoint

### 3.1 `POST /api/v1/checkin`

Record an owner check-in. Resets the inactivity timer.

**Request:** empty body.

**Response 200:**

```json
{
  "previous_state": "REMINDED_SOFT",
  "current_state": "ACTIVE",
  "last_checkin_at": "2026-05-09T12:34:56Z"
}
```

If the current state is `COOLING` or `FINAL_HOLD`, the response includes a `cancellation_confirmation` field acknowledging that the release was halted.

Also implicit on every authenticated request via middleware.

## 4. Vault entries endpoints

All endpoints under `/api/v1/vault/*` require authentication. See [Data Model section 4.4](04-data-model.md#44-vault_entries) for the `preview` / `bundle` storage shape.

### 4.1 `GET /api/v1/vault/entries`

List the user's entries. Returns `preview` blobs only.

**Query params:**

- `limit`: integer, default 100, max 500.
- `includeDeleted`: boolean, default false.

**Response 200:**

```json
{
  "entries": [
    {
      "id": "uuid-...",
      "preview": "base64-... (age-encrypted preview)",
      "sortOrder": 1,
      "schemaVersion": 1,
      "createdAt": "2026-05-09T12:34:56Z",
      "updatedAt": "2026-05-09T12:34:56Z",
      "deletedAt": null
    }
  ],
  "nextCursor": null
}
```

`nextCursor` is always `null`.

### 4.2 `POST /api/v1/vault/entries`

Create a new entry.

**Request:**

```json
{
  "preview": "base64-...",
  "bundle": "base64-...",
  "sortOrder": 1,
  "recipientContactIds": []
}
```

Empty `recipientContactIds` creates an owner-only entry. Non-empty list returns `400`.

**Response 201:** the created entry record including `bundle`.

### 4.3 `GET /api/v1/vault/entries/{id}`

Fetch one entry including `bundle`.

**Response 200:**

```json
{
  "id": "uuid-...",
  "preview": "base64-...",
  "bundle": "base64-...",
  "sortOrder": 1,
  "schemaVersion": 1,
  "createdAt": "2026-05-09T12:34:56Z",
  "updatedAt": "2026-05-09T12:34:56Z",
  "deletedAt": null
}
```

**Errors:** `404` if the entry does not exist or does not belong to the caller.

### 4.4 `PUT /api/v1/vault/entries/{id}`

Update an existing entry. Same request shape as create. Returns the updated record including `bundle`.

### 4.5 `DELETE /api/v1/vault/entries/{id}`

Soft-delete. Restorable within 30 days.

### 4.6 `POST /api/v1/vault/entries/{id}/restore`

Restore a soft-deleted entry. Returns the restored record. `404` if not found, not deleted, or past the 30-day window.

### 4.7 `POST /api/v1/vault/entries/reorder`

Bulk update of `sortOrder` for active entries.

**Request:**

```json
{
  "orders": [
    { "id": "uuid-...", "sortOrder": 100 },
    { "id": "uuid-...", "sortOrder": 200 }
  ]
}
```

**Response 204:** no body.

**Errors:** `400` if `orders` is empty or any id is malformed.

## 5. Contacts endpoints

### 5.1 `POST /api/v1/contacts`

Invite a new contact.

**Request:**

```json
{ "email": "alice@example.com", "display_name": "Alice" }
```

**Response 201:** pending contact record. Server emails the invitation link.

### 5.2 `GET /api/v1/contacts`

List the user's contacts and their states.

### 5.3 `POST /api/v1/contacts/{id}/approve`

Approve a contact after out-of-band fingerprint verification.

**Request:**

```json
{ "fingerprint_hash": "base64-..." }
```

The server checks that the submitted fingerprint matches the one stored when the contact registered. Mismatch returns 409.

### 5.4 `POST /api/v1/contacts/{id}/remove`

Mark a contact as removed. Triggers a UI prompt to reassign any entries currently assigned to this contact.

### 5.5 `GET /api/v1/onboarding/{token}`

Anonymous endpoint for an invitee to fetch their invitation context (owner display name, expiration).

### 5.6 `POST /api/v1/onboarding/{token}/register`

Anonymous endpoint where the invitee completes WebAuthn registration and submits their age recipient + fingerprint hash. After this, the contact is pending verification by the owner.

## 6. Release endpoints

### 6.1 `GET /api/v1/release/state`

Get the user's current release state.

**Response 200:**

```json
{
  "state": "ACTIVE",
  "last_checkin_at": "2026-05-09T...",
  "next_reminder_at": "2026-05-15T..."
}
```

### 6.2 `PUT /api/v1/release/offsets`

Update the user's release offsets (cadence). Bounded values per [Data Model section 4.8](04-data-model.md).

### 6.3 `POST /api/v1/release/{user_id}/false-positive`

Authenticated endpoint for the owner during the final hold to flag a false positive and revoke recipient access tokens.

### 6.4 `POST /api/v1/recover/{token}`

Recipient completes WebAuthn against the token (delivered via release-notification email); server returns ciphertexts assigned to that contact.

## 7. Audit log endpoints

### 7.1 `GET /api/v1/audit/entries`

List audit entries for the authenticated user.

**Query params:** `cursor`, `limit`, `since` (timestamp), `event_type`.

### 7.2 `POST /api/v1/audit/checkpoint`

Submit a signed checkpoint (the owner's browser signs the chain head).

**Request:**

```json
{
  "sequence": 1234,
  "chain_head_hash": "base64-...",
  "signature": "base64-..."
}
```

### 7.3 `GET /api/v1/audit/export`

Download the full audit log + checkpoints as a single JSON document for the owner's offline records.

## 8. Backup export

### 8.1 `POST /api/v1/backup/export`

Server assembles vault entries + contact metadata + audit chain head for the browser to encrypt.

**Request:**

```json
{ "include_passphrase_recipient": true }
```

**Response 200:** streaming bytes of the encrypted backup file.

### 8.2 `POST /api/v1/backup/import`

Server persists the decrypted vault state (replace or merge per UI choice).

## 9. Health

### 9.1 `GET /healthz`

Liveness probe. Returns `200` with JSON `{"status":"ok"}`. Outside the rate limiter.

### 9.2 `GET /readyz`

Readiness probe. Pings the database with a 2-second timeout. Returns `200` with `{"status":"ready"}` on success, or `503` with `{"status":"db unavailable","error":"..."}` on failure. Outside the rate limiter.

## 10. Test mode endpoints

Gated by `LGV_TEST_MODE=true`.

### 10.1 `POST /api/v1/test/fast-forward`

Advance the simulated clock by a duration. Used by E2E tests for the release state machine.

### 10.2 `POST /api/v1/test/email-inbox`

Return MailHog inbox contents for the test environment.

## 11. Rate limiting

One per-IP token bucket over `/api/*`. `/healthz` and `/readyz` are exempt.

- **Limit:** burst 10, then 1 token / 6 s (sustained 10/min per IP).
- **Client IP:** `RemoteAddr` only; no `X-Forwarded-For`. Terminate TLS on the same host.
- **IPv6:** bucketed by `/64` to block rotation bypass.
- **Idle cleanup:** entries unused for one hour are swept.
- **Exceeded:** `429` with `rate limit exceeded` body. No `Retry-After`.

No per-user bucket; passkey auth has no password to brute-force.
