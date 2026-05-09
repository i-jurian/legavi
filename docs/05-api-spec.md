# 05 - API Specification

**Update history**

- 2026-05-09: Initial draft

---

HTTP API. The OpenAPI source-of-truth lives at `backend/api/openapi.yaml`; this document explains design intent and consolidates the endpoint surface.

## 1. Conventions

- **Base path:** `/api/v1/...`. All endpoints are versioned.
- **Transport:** JSON over HTTPS. TLS 1.3 minimum.
- **Authentication:** JWT in the `Authorization: Bearer <token>` header for authenticated endpoints. Refresh token in an httponly cookie.
- **CSRF:** double-submit cookie. The `X-CSRF-Token` header must match the value in the `_csrf` cookie for state-changing requests.
- **Idempotency:** state-changing requests accept an `Idempotency-Key` header. The server stores recent keys and short-circuits duplicates.
- **Errors:** consistent error shape (see section 13).
- **Rate limits:** per-IP and per-user limits documented per endpoint group.
- **Content type:** request/response bodies are `application/json` unless noted.

## 2. Authentication endpoints

### 2.1 `POST /api/v1/auth/register/start`

Begin WebAuthn registration. Anonymous (no auth required).

**Request:**

```json
{
  "email": "user@example.com",
  "display_name": "User Name"
}
```

**Response 200:**

```json
{
  "challenge": "base64url-...",
  "rp": { "name": "Legavi", "id": "vault.example.com" },
  "user": {
    "id": "base64url-...",
    "name": "user@example.com",
    "displayName": "User Name"
  },
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
```

**Errors:**

- `400` if email is malformed.
- `409` if email is already registered (with constant-time delay to mitigate enumeration).

**Rate limit:** 10/min per IP.

### 2.2 `POST /api/v1/auth/register/verify`

Submit attestation to complete registration.

**Request:**

```json
{
  "challenge_id": "uuid-...",
  "credential_id": "base64url-...",
  "public_key": "base64url-... (COSE)",
  "attestation_object": "base64url-...",
  "client_data_json": "base64url-...",
  "age_recipient": "age1...",
  "transports": ["internal", "hybrid"]
}
```

**Response 200:**

```json
{
  "user_id": "uuid-...",
  "session": { "access_token": "eyJ...", "expires_in": 900 }
}
```

Refresh token is set in an httponly cookie (`refresh_token`).

**Errors:**

- `400` for invalid attestation, expired challenge, or signature mismatch.
- `409` if credential ID is already registered.

### 2.3 `POST /api/v1/auth/login/start`

Begin authentication. Anonymous.

**Request:**

```json
{ "email": "user@example.com" }
```

**Response 200:** WebAuthn `PublicKeyCredentialRequestOptions` with challenge and `allowCredentials` for the user.

For unknown emails, the response is constant-time-padded and indistinguishable from a real account challenge (mitigates email enumeration).

**Rate limit:** 30/min per IP.

### 2.4 `POST /api/v1/auth/login/verify`

Submit assertion to complete authentication.

**Request:**

```json
{
  "challenge_id": "uuid-...",
  "credential_id": "base64url-...",
  "authenticator_data": "base64url-...",
  "client_data_json": "base64url-...",
  "signature": "base64url-..."
}
```

**Response 200:** same shape as `register/verify`.

**Errors:**

- `400` for invalid signature.
- `401` if signature doesn't match stored public key.

### 2.5 `POST /api/v1/auth/refresh`

Rotate access token using refresh cookie.

**Response 200:**

```json
{ "access_token": "eyJ...", "expires_in": 900 }
```

A new refresh token replaces the old one in the cookie. Old refresh token is invalidated server-side.

### 2.6 `POST /api/v1/auth/logout`

Invalidate refresh token, clear cookies.

**Response 204.**

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

This endpoint is also implicitly called on every authenticated request via middleware; the explicit endpoint exists for clients that want to check in without performing another action.

## 4. Vault entries endpoints

All endpoints under `/api/v1/vault/*` require authentication.

### 4.1 `GET /api/v1/vault/entries`

List the user's entries. Returns ciphertext + metadata; the browser decrypts on demand.

**Query params:**

- `limit`: integer, default 100, max 500.
- `cursor`: opaque pagination cursor.
- `include_deleted`: boolean, default false.

**Response 200:**

```json
{
  "entries": [
    {
      "id": "uuid-...",
      "label_hint": "Bank credentials",
      "sort_order": 1,
      "ciphertext": "base64-... (age-encrypted zip bundle)",
      "schema_version": 1,
      "recipients": [
        {
          "contact_id": "uuid-...",
          "display_name": "Alice",
          "age_recipient": "age1..."
        }
      ],
      "created_at": "2026-05-09T...",
      "updated_at": "2026-05-09T...",
      "deleted_at": null
    }
  ],
  "next_cursor": null
}
```

### 4.2 `POST /api/v1/vault/entries`

Create a new entry.

**Request:**

```json
{
  "label_hint": "Bank credentials",
  "ciphertext": "base64-...",
  "sort_order": 1,
  "recipient_contact_ids": ["uuid-..."]
}
```

The browser zips the user's selected files in memory and age-encrypts the bundle to the recipient set (owner + zero or more assigned contacts). The server stores the opaque ciphertext and validates that all `recipient_contact_ids` are verified contacts of the user. An empty `recipient_contact_ids` array creates an owner-only entry that never enters the release flow.

**Response 201:** the created entry record.

### 4.3 `PUT /api/v1/vault/entries/{id}`

Update an existing entry. Same shape as create.

If `recipient_contact_ids` changes, the browser MUST re-encrypt and submit fresh ciphertext for the new recipient set. The server enforces that the recipient list in the request matches the recipients embedded in the age header (a basic consistency check; the cryptographic enforcement is age's own).

### 4.4 `DELETE /api/v1/vault/entries/{id}`

Soft-delete an entry. Restorable within 30 days.

### 4.5 `POST /api/v1/vault/entries/{id}/restore`

Restore a soft-deleted entry within the 30-day window.

### 4.6 `POST /api/v1/vault/entries/bulk-reassign`

Bulk reassign recipients across multiple entries. Browser submits a list of `(entry_id, new_ciphertext, new_recipient_contact_ids)` tuples. Server applies in a single transaction.

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

Anonymous endpoint for a designated recipient to retrieve their assigned ciphertexts after release fires. The token is delivered via the recipient's release-notification email. The recipient completes a WebAuthn ceremony and the server returns the ciphertexts they're assigned to.

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

Generate an age-encrypted backup. The browser performs the encryption; the server only assembles the input (vault entries + contact metadata + audit chain head).

**Request:**

```json
{ "include_passphrase_recipient": true }
```

**Response 200:** streaming bytes of the encrypted backup file.

### 8.2 `POST /api/v1/backup/import`

Import a backup. The browser decrypts; the server receives the new vault state and writes it (replacing or merging per the user's choice in the UI).

## 9. Health

### 9.1 `GET /healthz`

Liveness probe. Returns `200 OK` always (process is responsive).

### 9.2 `GET /readyz`

Readiness probe. Returns `200 OK` when DB connection is healthy; `503` otherwise.

A `/metrics` endpoint is deferred to M7.

## 10. CSRF endpoint

### 10.1 `GET /api/v1/csrf`

Issue a fresh CSRF token. Sets the `_csrf` cookie and returns the same value in the response body. Browser sends the value as `X-CSRF-Token` on subsequent state-changing requests.

## 11. Test mode endpoints

These endpoints exist only when `LGV_TEST_MODE=true`. The server refuses to start with this flag in a production-detected environment.

### 11.1 `POST /api/v1/test/fast-forward`

Advance the simulated clock by a duration. Used by E2E tests for the release state machine.

### 11.2 `POST /api/v1/test/email-inbox`

Return MailHog inbox contents for the test environment.

## 12. Rate limiting

Per-IP and per-user. Limits documented per endpoint where they deviate from the default. Default: 100 requests/minute per authenticated user, 60 requests/minute per anonymous IP.

Rate-limit responses use `429 Too Many Requests` with `Retry-After` header.

## 13. Error format

```json
{
  "error": {
    "code": "INVALID_RECIPIENT",
    "message": "Recipient is not a verified contact of this user.",
    "details": { "contact_id": "uuid-..." }
  },
  "request_id": "uuid-..."
}
```

All error responses include the request correlation ID. The `code` is a stable string for client-side handling; `message` is human-readable English; `details` is optional structured context.
