# 03 - System Architecture

**Update history**

- 2026-05-09: Initial draft

---

## 1. Component diagram

```mermaid
flowchart TB
    Browser[Browser SPA] -->|HTTPS / JSON| Caddy[Caddy<br/>reverse proxy + TLS]

    subgraph Backend["Backend"]
        API[API Server]
        Scheduler[Scheduler]
        Worker[Worker]
    end

    Caddy --> API

    API <--> DB[(Postgres)]
    Scheduler <--> DB
    Worker <--> DB
    Worker -.->|SMTP| Mail([MailHog, dev SMTP])
```

Component responsibilities are detailed below. In production, MailHog is replaced with an SMTP relay.

## 2. Component responsibilities

### 2.1 Frontend SPA

- WebAuthn ceremonies for registration and authentication.
- PRF extraction and age identity derivation. Identity exists in browser memory only.
- All vault encryption and decryption. Server never sees plaintext.
- Per-entry recipient assignment UI. On assignment changes, re-encrypts the affected entries client-side.
- Audit log chain verification on session start.
- UI for vault entries, contact onboarding, release-policy configuration.

### 2.2 API Server

- HTTP request handling.
- WebAuthn server-side ceremony (challenge generation, signature verification).
- Stores ciphertext, public credentials, per-entry recipient assignments.
- Issues JWTs for session authentication.
- Refuses any request that would require it to see plaintext.

### 2.3 Scheduler

- Single-leader via Postgres advisory lock.
- Ticks every 60 seconds.
- For each owner: evaluates state machine transitions based on last check-in, configured offsets, current state.
- Emits state-change events to the worker queue.

### 2.4 Worker

- Consumes events from the queue.
- Sends emails (reminders, contact invitations, release notifications).
- Retries with exponential backoff on transient failures.
- Idempotent per event ID.

### 2.5 Postgres

- Single primary with nightly backups.
- Stores: users, credentials (passkey public keys), vault entries (ciphertext + per-entry recipient assignments), contacts, release state, audit log, scheduling state, jobs.
- Schema documented in [Data Model](04-data-model.md).

### 2.6 Caddy

- TLS termination, HTTP→HTTPS redirect.
- Security headers (HSTS, CSP, X-Frame-Options, Referrer-Policy).
- Edge rate limiting.

## 3. Data flow: owner login

```
1. Browser fetches /api/v1/auth/challenge with email.
2. Server returns a signed challenge nonce.
3. Browser invokes navigator.credentials.get(...) with PRF extension.
4. User completes biometric / PIN ceremony in their authenticator.
5. Browser receives signature + PRF output.
6. Browser derives age identity from PRF output; retains in memory.
7. Browser POSTs signature + credentialId to /api/v1/auth/verify.
8. Server verifies signature, issues JWT in response body and refresh token in httponly cookie.
9. Browser fetches encrypted vault entries (paginated).
10. For each viewed entry, browser age-decrypts using the retained identity.
```

## 4. Data flow: contact onboarding

```
1. Owner submits contact email and display name.
2. Server creates a pending contact record, sends invitation email with a one-time link.
3. Contact clicks link, lands on /onboarding/<token>.
4. Browser starts WebAuthn registration ceremony with PRF.
5. Contact completes biometric / PIN.
6. Browser receives PRF output, derives contact's age recipient.
7. Browser submits credentialId, public key, age recipient, and a fingerprint hash to the server.
8. Server stores the contact's credentials in pending state.
9. Owner is notified, displays the contact's recipient fingerprint.
10. Owner verifies fingerprint with contact via known-good out-of-band channel (call, in person).
11. Owner approves, server marks contact as verified.
12. The contact is now eligible to be assigned to vault entries. Owner can assign existing entries to this contact (browser re-encrypts each affected entry to the new recipient set) or assign on entry creation going forward.
```

## 5. Data flow: release fires

```
1. Scheduler tick observes that owner has not checked in past the final offset.
2. Scheduler transitions release state to COOLING. Worker emails each contact assigned to at least one entry: "X has not checked in. Release is in cooling period."
3. 48 hours pass with no check-in. Scheduler transitions to FINAL_HOLD. Worker emails owner: "Cooling period expired. 24-hour final hold begins. Flag false-positive in the app to cancel."
4. 24 hours pass with no check-in and no false-positive flag. Scheduler transitions to RELEASED.
5. Worker emails each contact assigned to at least one entry a release notification with a link to /recover/<id>.
6. Recipient visits the link, completes WebAuthn authentication.
7. Recipient's browser derives their age identity from PRF.
8. Recipient's browser fetches the ciphertext for entries assigned to them.
9. Recipient's browser age-decrypts each entry using their identity. Display.
```

## 6. Trust boundaries

See [Threat Model section 1](01-threat-model.md) for the full trust diagram. The server holds ciphertext, public credentials, scheduling state, and recipient-assignment metadata, but is never a recipient on any entry, so server compromise yields no plaintext.

## 7. Configuration

Twelve-factor: every parameter is an environment variable.

| Variable                       | Default                            | Purpose                                                                                       |
| ------------------------------ | ---------------------------------- | --------------------------------------------------------------------------------------------- |
| `LGV_DATABASE_URL`             | `postgres://...`                   | Postgres connection string                                                                    |
| `LGV_PUBLIC_URL`               | `https://localhost:8080`           | Public URL for WebAuthn rpID and email links                                                  |
| `LGV_RP_ID`                    | derived from `LGV_PUBLIC_URL` host | WebAuthn relying party ID                                                                     |
| `LGV_JWT_SIGNING_KEY`          | required                           | HMAC key for JWT signing                                                                      |
| `LGV_SMTP_URL`                 | `smtp://mailhog:1025`              | SMTP for outbound mail                                                                        |
| `LGV_SCHEDULER_LEADER_LOCK_ID` | `42`                               | Postgres advisory lock ID for leader election                                                 |
| `LGV_TEST_MODE`                | `false`                            | Enables test-only endpoints (clock fast-forward, etc.); rejected at startup if production-ish |
| `LGV_LOG_LEVEL`                | `info`                             | `debug`, `info`, `warn`, `error`                                                              |

## 8. Deployment topology

Target deployment: solo self-hosted on a single VM running Docker Compose with all services on one host.

## 9. Observability

- Structured JSON logs, one event per line, with correlation ID per request.
- Metrics endpoint deferred to M7; tool choice (Prometheus, OpenTelemetry, or alternative) open until then.
- Healthcheck endpoints per [API Spec section 9](05-api-spec.md).
- Logs avoid plaintext secrets and vault contents; user and contact records are referenced by ID rather than email where it does not hurt debuggability.

## 10. Schema migration strategy

Database migrations via `goose`; details in [Data Model](04-data-model.md).

Wire format migrations (changes to encrypted blob layouts) follow the `schema_version` rule from [Crypto Spec section 11](02-crypto-spec.md): readers reject unknown versions, blobs include version in their associated data.
