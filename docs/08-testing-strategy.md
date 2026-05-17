# 08 - Testing Strategy

Target posture. See [IMPLEMENTATION_STATUS.md](IMPLEMENTATION_STATUS.md) for current state.

## 1. Coverage targets

Crypto code extracts into a dedicated package (see [Crypto Spec section 12](02-crypto-spec.md)).

| Package                          | Target                 | Rationale                       |
| -------------------------------- | ---------------------- | ------------------------------- |
| Backend `crypto/`                | 100% line, 100% branch | Every line is security-critical |
| Frontend `src/crypto/`           | 100% line, 100% branch | Same                            |
| `release/` (state machine)       | >= 95%                 | Operational criticality         |
| `api/` handlers                  | >= 85%                 | Standard backend                |
| `frontend/` components           | >= 70%                 | Lower bar; supplemented by E2E  |
| `worker/`, `scheduler/`          | >= 80%                 |                                 |

PRs that drop coverage below the floor are blocked.

## 2. Crypto testing

### 2.1 Roundtrip and recipient correctness

For age multi-recipient encryption:

- **Owner-only roundtrip:** owner encrypts to `[owner]`; owner decrypts. No other party (alice, bob, server) can.
- **Single-recipient roundtrip:** owner encrypts to `[owner, alice]`, both owner and alice can decrypt independently. Bob (not in recipients) cannot.
- **Group roundtrip:** owner encrypts to `[owner, alice, bob]`, all three can decrypt independently. Charlie (not in recipients) cannot.
- **Reassignment correctness:** entry encrypted to `[owner, alice]`, then re-encrypted to `[owner, bob]`. Old ciphertext (if retained for testing) is still alice-decryptable; new ciphertext is bob-decryptable, not alice-decryptable.

### 2.2 Property-based tests

- age multi-recipient: any recipient in the set decrypts; any recipient outside the set fails.
- Ed25519 sign/verify: for any keypair and any message, verify(sign) returns true.
- Ed25519 negative: tampering with the message or the signature causes verify to return false.
- Audit chain: any modification to entry `n` invalidates entry `n+1`'s `prev_entry_hash`.

### 2.3 PRF determinism

Test that the same passkey produces the same age recipient on every authentication ceremony (within the same session and across sessions). Requires either a fixture authenticator (in unit tests) or a real authenticator (in E2E).

### 2.4 Forbidden patterns

A linter rule scans for and rejects:

- `math/rand` import in production code.
- `Math.random()` in production code.
- AES-CBC, AES-ECB, MD5, SHA-1.
- Any string literal that looks like a hardcoded key (high-entropy detection).
- Direct invocation of low-level age primitives (X25519, ChaCha20-Poly1305) outside of the age library and the deliberate `deriveAgeKeypair` helper.

## 3. State machine testing

The release state machine ([Release Orchestration](06-release-orchestration.md)) is exercised by:

### 3.1 Pure-function unit tests

Table-driven test exercises `ComputeNextState` against every transition in the state diagram, asserting the expected next state for each `(currentState, lastCheckin, offsets, now)` tuple.

### 3.2 Integration test with fake clock

Drive the scheduler with a fake clock through the full `ACTIVE` -> `RELEASED` path; assert state and emitted-email count at each offset boundary.

### 3.3 Property tests

- **Idempotency:** calling `Tick()` repeatedly without time advance produces no state changes after the first.
- **Monotonicity:** state never moves backward except via `checkin` or `is_false_positive` (the latter only during the 24-hour final hold).
- **No release before final offset elapses, ever.**
- **After `final_hold_until` has elapsed in the RELEASED state, no transition back to ACTIVE is possible.**
- **Precondition gating:** an owner with zero contact-assigned entries stays in `ACTIVE` regardless of `last_checkin_at` age; making one contact assignment engages the state machine; removing the last contact assignment disengages it.

### 3.4 Chaos / failure injection

Kill the scheduler mid-transition and restart it: assert no duplicate notifications. Concurrent-scheduler: two instances starting in the same tick window; only the advisory-lock holder fires jobs.

### 3.5 Final-hold and false-positive scenarios

Specific tests for the cancel-during-final-hold path:

- User check-in at `final_hold_until - 1h` flags false-positive, revokes recipient tokens, returns to ACTIVE.
- User check-in at `final_hold_until + 1h` does NOT cancel; release stands.
- A recipient who fetched their ciphertext before the cancel does not get auto-revoked from their browser cache (best-effort; documented in audit log).
- Scheduler restart during the final hold does not double-fire the recipient notification email.

## 4. API testing

### 4.1 Handler tests

For every endpoint, a test for:

- Happy path (valid input, expected output).
- Each documented error condition.
- Authentication enforcement (anonymous -> 401).
- Authorization enforcement (other user's data -> 404 not 403, to prevent enumeration).
- Recipient-validation enforcement (assign-to-non-verified-contact -> 400).

DB isolation: each test runs inside a transaction that rolls back on completion. A shared Postgres service container is reused across tests for speed; per-test rollback prevents leakage.

### 4.2 WebAuthn ceremony tests

WebAuthn ceremonies are tested with a fixture authenticator (`@simplewebauthn/server`'s test helpers) to simulate registration and authentication without a real device.

## 5. Frontend testing

### 5.1 Component tests

Every form component has a test for:

- Renders with default props.
- Validation triggers on invalid input.
- Submission calls the right mutation with the right shape.
- Loading and error states render correctly.

### 5.2 Crypto session tests

The `CryptoSession` class is tested for:

- Locks after 5min inactivity.
- Locks on visibility change (after 60s).
- Throws on access while locked.
- Best-effort zeroing of the identity on lock (verified via `Uint8Array.fill(0)` having been called).

### 5.3 E2E tests

Critical paths:

1. **Full registration** - passkey ceremony with fixture authenticator, set up first vault entry.
2. **First contact invite + acceptance** - uses two browser contexts (inviter and invitee).
3. **Vault entry CRUD with reassignment** - create, edit, reassign recipients, delete, restore.
4. **Fingerprint verification** - UI flow with code displayed and confirmed.
5. **Simulated release** - uses test-only `/api/v1/test/fast-forward` endpoint to advance scheduler clock; verifies email at MailHog and recipient retrieval flow.
6. **Final-hold cancel** - release fires; user logs in within 24h with false-positive flag; verifies tokens revoked.
7. **Final-hold expiry** - release fires; no user activity for 25h; verifies recipients can decrypt.

Tests run against a containerized stack via `docker compose -f compose.test.yaml`.

### 5.4 Accessibility

One Playwright E2E run with `@axe-core/playwright` as a smoke test. Failures block CI.

## 6. Security testing

### 6.1 Static analysis

- `go vet`, `staticcheck` on every push.
- `eslint` with `eslint-plugin-security` on TS code.

### 6.2 Dependency scanning

- `govulncheck` on every push.
- `npm audit --audit-level=high` on every push.
- Manual dependency review when advisories surface; PR-based bumps reviewed against the spec and re-run KAT/parity tests before merge.

## 7. CI pipeline

`.github/workflows/ci.yml` runs three jobs on every push and pull request: `lint` (golangci-lint, eslint, tsc), `unit` (go test, vitest), `security-audit` (govulncheck, npm audit). A PR is mergeable when all pass.

## 8. Test data

- **Never use real user data for testing.** All fixtures are synthetic.
- **Faker-generated** names and emails; deterministic seeds for reproducibility.
- **No production data ever copied to development environments.**
- `LGV_TEST_MODE=true` enables E2E-only endpoints (see [API Spec section 10](05-api-spec.md)).
