# 03 - System Architecture

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

In production, MailHog is replaced with an SMTP relay.

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
- Sets `lgv_session` (JWT, httponly, `SameSite=Strict`) on register/login success.
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
- Data access via `pgx` pool + `sqlc`-generated typed queries from `internal/store/queries/*.sql`, wrapped by handwritten `store.Store` methods.

### 2.6 Caddy

- TLS termination, HTTP->HTTPS redirect.
- Security headers (HSTS, CSP, X-Frame-Options, Referrer-Policy).
- Edge rate limiting.

## 3. Data flow: owner login

```
1.  Browser POSTs email to /api/v1/auth/login/start.
2.  Server returns CredentialAssertion (wrapped as { publicKey: ... }) with the PRF salt, and sets the lgv_webauthn_session ceremony cookie.
3.  Browser calls navigator.credentials.get(...) via @simplewebauthn/browser.
4.  User completes biometric / PIN ceremony.
5.  Browser receives the assertion plus the PRF output.
6.  Browser stores PRF bytes in CryptoSession; age keypair is derived on demand.
7.  Browser POSTs the full AuthenticationResponseJSON to /api/v1/auth/login/verify.
8.  Server verifies the assertion, sets the lgv_session JWT cookie, clears the ceremony cookie. Empty body.
9.  Browser fetches encrypted vault entries.
10. Each viewed entry is age-decrypted in the browser.
```

## 4. Data flow: contact onboarding

See [Crypto Spec section 7](02-crypto-spec.md#7-contact-onboarding). The API server stores the contact's credentials in a pending state until the owner approves the fingerprint.

## 5. Data flow: release fires

Scheduler runs the state machine per [Release Orchestration section 3](06-release-orchestration.md). On entering `RELEASED`:

```
1. Worker emails each assigned contact a release notification with /recover/<id>.
2. Recipient visits the link, completes WebAuthn authentication.
3. Recipient's browser derives their age identity from PRF.
4. Browser fetches assigned ciphertext and age-decrypts each entry locally.
```

## 6. Configuration

Twelve-factor: every parameter is an environment variable. All variables below are required at startup; no compiled-in defaults.

| Variable              | Example                                                         | Purpose                                                          |
| --------------------- | --------------------------------------------------------------- | ---------------------------------------------------------------- |
| `LGV_DATABASE_URL`    | `postgres://legavi:legavi@postgres:5432/legavi?sslmode=disable` | Postgres connection string                                       |
| `LGV_PUBLIC_URL`      | `http://localhost:8080`                                         | Public URL; WebAuthn RP ID is its hostname, origin is the URL    |
| `LGV_JWT_SIGNING_KEY` | output of `openssl rand -base64 32`                             | HMAC key for the session JWT; decodes to >= 32 bytes             |
| `LGV_JWT_TTL`         | `24h`                                                           | Lifetime of `lgv_session`; any `time.ParseDuration` value        |
| `LGV_API_LISTEN`      | `:8080`                                                         | Bind address                                                     |
| `LGV_SMTP_URL`        | `smtp://mailhog:1025`                                           | SMTP for outbound mail                                           |
| `LGV_FROM_EMAIL`      | `noreply@localhost`                                             | From address on outbound mail                                    |
| `LGV_LOG_LEVEL`       | `info`                                                          | `debug`, `info`, `warn`, `error`                                 |
| `LGV_TEST_MODE`       | `false`                                                         | Enables test-only endpoints (clock fast-forward)                 |

## 7. Observability

- Structured JSON logs, one event per line, with correlation ID per request.
- Healthcheck endpoints per [API Spec section 9](05-api-spec.md).
- Logs avoid plaintext secrets and vault contents; user and contact records are referenced by ID rather than email where it does not hurt debuggability.
