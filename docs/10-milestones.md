# 10 - Milestones

**Update history**

- 2026-05-09: Initial draft

---

Each milestone has a narrow objective and a clear exit criterion.

## Milestone 0 - Scaffold

The full development scaffold: dev environment, Go backend skeleton, frontend Vite scaffold, CI, and pre-commit hooks.

1. Repo skeleton folders: `backend/`, `frontend/`, `deploy/docker/`, `docs/`, `.github/`.
2. Root `Makefile` with: `dev`, `dev-down`, `test`, `test-be`, `test-fe`, `lint`, `lint-be`, `lint-fe`.
3. Docker Compose for dev: `postgres:16`, `mailhog`, `caddy`. Backend developed via `go run` against this stack.
4. Backend skeleton:
   - `cmd/api/main.go`, `cmd/scheduler/main.go`, `cmd/worker/main.go`. Each prints a structured log line and exits gracefully on signal.
   - Postgres migrations 1-3 per [Data Model section 3](04-data-model.md), applied with `goose`.
   - Config loader (env-based, twelve-factor) per [Architecture section 7](03-architecture.md).
   - Health endpoints `/healthz` (liveness) and `/readyz` (readiness, checks DB connection).
   - Structured JSON logging with correlation IDs.
   - Chi router setup with middleware composition.
5. Frontend scaffold:
   - Vite + React + TypeScript + Tailwind + shadcn.
   - TanStack Router (`@tanstack/react-router`) skeleton with placeholder routes per [Frontend Spec section 2](07-frontend-spec.md).
   - TanStack Query setup; `authStore` (Zustand) skeleton.
   - `CryptoSession` singleton skeleton (lock-on-idle / visibility-change wiring; not yet used for crypto).
   - Bundle SRI enabled in the Vite build (e.g., `vite-plugin-sri`).
   - `@simplewebauthn/browser` and `age-encryption` dependencies installed (not yet imported).
6. CI: GitHub Actions workflows for `lint` (`golangci-lint`, `eslint`), `unit` (`go test`, `vitest`), `security-audit` (`govulncheck`, `npm audit`).
7. Pre-commit hooks via `lefthook`: `gofmt` on Go side, `eslint` on TS side.

**Exit:** `make dev` brings up Postgres + MailHog + Caddy; `go run ./cmd/api` responds to `/healthz`; the Vite dev server loads a placeholder page; CI green on the scaffold commits.

**Deferred to M7 (or first external PR):** issue and PR templates, conventional-commit hook.

## Milestone 1 - WebAuthn authentication

1. Server endpoints per [API Spec section 2](05-api-spec.md).
2. WebAuthn server library: `github.com/go-webauthn/webauthn`.
3. PRF extension passthrough: server forwards PRF data unchanged; the browser computes PRF output during the ceremony and derives the age identity.
4. Browser-side ceremonies via `@simplewebauthn/browser`. Login, register, logout flows wired end-to-end through the API. `CryptoSession` holds the derived age identity for the session.
5. Rate limiting middleware on auth endpoints.
6. Tests: handler-level (Go) for happy path and each documented error condition; component- and E2E-level (Vitest + Playwright) for the browser ceremonies with a fixture authenticator.

**Exit:** a user can register a passkey via browser, log in, and see a placeholder dashboard. No vault yet.

## Milestone 2 - Vault entries

1. `vault_entries` CRUD endpoints per [API Spec section 4](05-api-spec.md). Server stores opaque ciphertext + metadata only.
2. Frontend vault UI: list, create, edit (add/remove files), delete, restore (within soft-delete window).
3. Browser-side zip via `@zip.js/zip.js`; entry payload is a zip bundle of one or more files.
4. Encryption/decryption via the `age-encryption` npm package, scoped to the owner's age identity from the `CryptoSession`.
5. `label_hint` and `sort_order` UI.

**Exit:** a single user can store and retrieve encrypted vault entries via the browser.

## Milestone 3 - Contacts and per-entry recipient assignment

1. Contact invitation flow: owner submits email + name, server emails invitation link with a one-time token.
2. Contact onboarding page: WebAuthn registration ceremony, age recipient computed from PRF, fingerprint hash submitted.
3. Out-of-band fingerprint verification UI: display fingerprint to owner, owner reads it to contact via known-good channel, owner confirms.
4. Per-entry recipient assignment UI: assign or reassign entries to one or more contacts, with bulk assignment support.
5. Re-encryption flow: when an entry's recipient list changes, the browser fetches ciphertext, decrypts with owner identity, re-encrypts to the new recipient set, uploads.
6. UI for viewing contacts, their state (pending, verified, removed), the entries currently assigned to each, and re-running fingerprint verification.

**Exit:** owner can invite contacts, complete the verification flow, assign entries to contacts, and reassignment correctly re-encrypts affected entries.

## Milestone 4 - Release state machine

1. Scheduler implementation: single-leader via Postgres advisory lock, ticks every 60 seconds.
2. Pure-function state transition logic, exhaustively unit-tested per [Testing Strategy section 3](08-testing-strategy.md).
3. Worker queue: emits emails on state transitions. Idempotent per event ID.
4. Reminder cadence (configurable defaults: 7d soft, 14d firm, 30d final).
5. 48-hour firm cooling period.
6. 24-hour final hold with false-positive cancel flag.
7. Test-only `LGV_TEST_MODE=true` endpoints for clock fast-forward in E2E tests.

**Exit:** simulated 30-day inactivity completes release correctly in an E2E test; check-in correctly cancels at any point before the final hold expires.

## Milestone 5 - Audit log and backup export

1. Hash-chained audit log entries with sequence numbers.
2. Owner-signed Ed25519 checkpoints at login, release transitions, and 24h activity boundaries.
3. Owner UI: view audit log, verify chain integrity, download log.
4. Backup export: age-encrypted file with owner-recipient and optional passphrase-recipient.
5. Backup import on a new device: restore vault using passphrase or current passkey.

**Exit:** owner can view audit log with chain verification status; can export and re-import an encrypted backup.

## Milestone 6 - Design polish

1. Visual identity: accent color, palette locked into Tailwind / CSS variables, logo or wordmark.
2. Cross-feature consistency pass: layout shell, navigation pattern, spacing rhythm.
3. Empty, error, and loading states for every screen.
4. Microcopy pass: error messages, button labels, confirmation dialogs.
5. Accessibility second pass: keyboard navigation, focus-visible styles, contrast, screen-reader labels on icon-only controls.
6. Screenshot pass for the README.

**Exit:** every screen has explicit empty, error, and loading states; visual identity is consistent across features.

## Pre-MVP checks

Run before starting M7:

- [ ] Roundtrip and recipient-correctness tests green in CI.
- [ ] Property-based tests green.
- [ ] Forbidden patterns trigger linter failures (verified by intentional violations).
- [ ] `govulncheck` and `npm audit` clean.
- [ ] All TODO/FIXME comments in security-relevant paths resolved or documented.

## Milestone 7 - Deployment, docs

1. Production Docker image build, distroless base, multi-stage build.
2. Caddyfile for production (real domain, real cert, security headers).
3. SMTP relay configuration for production email.
4. Observability: pick a metrics tool, wire up the endpoint or exporter, document scraper setup for self-hosters.
5. Issue and PR templates under `.github/`.
6. Conventional-commit hook (lefthook commit-msg).
7. Deployment guide walked end-to-end on a clean VM.
8. Public-facing documentation polish.

**Exit:** clean-VM deployment in under 60 minutes per documented procedure. CI green across all gates.
