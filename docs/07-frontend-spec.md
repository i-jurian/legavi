# 07 - Frontend Specification

**Update history**

- 2026-05-09: Initial draft

---

## 1. Stack

- **Vite + React 18 + TypeScript** (strict mode).
- **Tailwind CSS + shadcn/ui** for styling.
- **TanStack Router** (`@tanstack/react-router`) for routing.
- **TanStack Query** for server-state caching.
- **Zustand** for client state.
- **React Hook Form + Zod** for forms and validation.
- **`@simplewebauthn/browser`** for WebAuthn ceremonies.
- **`age-encryption`** (npm) for age encryption/decryption.
- **`@zip.js/zip.js`** for browser-side zip/unzip of vault bundles.
- **`@noble/curves/ed25519`** for audit log signatures.

## 2. Routes

| Path                 | Auth required? | Purpose                                                   |
| -------------------- | -------------- | --------------------------------------------------------- |
| `/`                  | No             | Marketing page (or redirect to `/dashboard` if logged in) |
| `/register`          | No             | Passkey registration ceremony                             |
| `/login`             | No             | Passkey authentication ceremony                           |
| `/dashboard`         | Yes            | Vault overview, recent activity                           |
| `/vault`             | Yes            | Vault entry list                                          |
| `/vault/new`         | Yes            | Create entry                                              |
| `/vault/:id`         | Yes            | View / edit entry                                         |
| `/contacts`          | Yes            | Contact list and management                               |
| `/contacts/new`      | Yes            | Invite a contact                                          |
| `/contacts/:id`      | Yes            | Contact detail (verification, assignments)                |
| `/release`           | Yes            | Release-policy configuration, current state               |
| `/audit`             | Yes            | Audit log viewer                                          |
| `/backup`            | Yes            | Export / import backup                                    |
| `/settings`          | Yes            | Account settings, device management                       |
| `/onboarding/:token` | No             | Invitee registration page                                 |
| `/recover/:token`    | No             | Recipient recovery page (post-release)                    |

## 3. Onboarding flow

```
Step 1   Email invite arrives.
Step 2   Click link, /onboarding/:token loads.
Step 3   Page explains: "<owner> wants to designate you as a recipient. You'll register with your device's biometric. No password to remember."
Step 4   Click "Continue with Face ID" (or platform-equivalent label).
Step 5   WebAuthn registration ceremony. Browser computes age recipient.
Step 6   Page displays a fingerprint code: "Read this 6-word code to <owner> over phone or in person."
Step 7   Done. Page shows: "<owner> will verify the code and approve you. You'll get a confirmation email."
```

For the owner's verification side:

```
Step 1   Owner sees a notification: "<contact> has registered. Verify their fingerprint."
Step 2   Owner reads the displayed fingerprint code, compares with what the contact reads back.
Step 3   Click "Confirm" or "Codes don't match" (which initiates a re-registration).
Step 4   On confirm, the contact appears as verified in the contact list.
```

## 4. Crypto session lifecycle

The `CryptoSession` is a Zustand-backed singleton managing the user's age identity in browser memory.

### 4.1 Lifecycle states

| State       | Description                                                | Identity in memory? |
| ----------- | ---------------------------------------------------------- | ------------------- |
| `LOCKED`    | No active session, no identity.                            | No                  |
| `UNLOCKING` | Mid-WebAuthn-ceremony, identity not yet derived.           | No                  |
| `UNLOCKED`  | Active session, identity available for decrypt operations. | Yes                 |
| `LOCKING`   | Lock-in-progress (zeroing identity).                       | Briefly, then no    |

### 4.2 Lock triggers

- **Idle timeout:** 5 minutes of no user interaction.
- **Visibility change:** tab loses focus for 60 seconds (i.e., user switched tabs/apps).
- **Explicit logout:** user clicks Logout.
- **Server signaled session expiry:** access token rejected, refresh failed.

On lock, the in-memory age identity is zeroed via `array.fill(0)`. Best-effort; JavaScript offers no stronger guarantee against memory residue.

## 5. WebAuthn ceremonies

### 5.1 Registration

Sketch (illustrative; actual API names depend on the library version):

```typescript
import { startRegistration } from "@simplewebauthn/browser";
// import age helpers from age-encryption per its docs

const challenge = await fetch("/api/v1/auth/register/start", { ... }).then(r => r.json());
const credential = await startRegistration({ optionsJSON: challenge });

// Extract the 32-byte PRF output from credential.clientExtensionResults.prf.results.first
// Construct an age X25519 identity from those bytes (per age-encryption docs).
// Derive the corresponding age recipient.

await fetch("/api/v1/auth/register/verify", {
  method: "POST",
  body: JSON.stringify({
    challenge_id: challenge.id,
    credential_id: credential.id,
    public_key: credential.response.publicKey,
    attestation_object: credential.response.attestationObject,
    client_data_json: credential.response.clientDataJSON,
    age_recipient: /* derived recipient string */,
    transports: credential.response.transports,
  }),
});
```

Building the age identity from PRF output requires clamping the 32 bytes into a valid X25519 private key. The `age-encryption` npm package exposes the right primitives; the exact entry point should be confirmed against the installed version.

### 5.2 Authentication

Similar to registration but using `startAuthentication`. The PRF output recomputes deterministically, yielding the same age identity.

## 6. Vault UI

### 6.1 Entry list

Sortable, filterable by assignment and label. Each row shows:

- Label hint
- File count
- Assigned to (avatars or names of designated contacts; empty / "-" if owner-only)
- Last updated timestamp
- Quick actions: open, edit, reassign, delete

The "Assigned to" column reflects only the contacts the owner has designated. The owner is always a cryptographic recipient on every entry (so they can decrypt their own data) but is not displayed here.

Decryption happens lazily: rows display ciphertext metadata until the user clicks to open; clicking decrypts and unzips in memory and shows the bundle's file list.

### 6.2 Create / edit entry

An entry is a bundle of files. Form with:

- Label hint (plaintext, server-visible)
- Files (drag-drop or file picker; multiple files allowed; default cap 25 MB total per bundle)
- Assigned to (multi-select from verified contacts; optional; empty = owner-only entry, never enters the release flow)

On submit:

1. Browser derives current age identity from session (must be `UNLOCKED`).
2. Browser computes the recipient set: `[owner_recipient, ...selected_contact_recipients]`.
3. Browser zips the selected files into an in-memory bundle via `@zip.js/zip.js`.
4. Browser age-encrypts the zip bundle.
5. Browser POSTs ciphertext + label hint + recipient IDs.

On view: browser age-decrypts the ciphertext, unzips in memory, and renders a file list. Each file has inline preview where possible (images, PDFs, text, video, audio via blob URLs scoped to the page) and a download button as fallback. Office documents and unrecognized types are download-only. Blob URLs are revoked when the entry view closes so decrypted bytes can be garbage-collected.

On edit (add/remove files): browser fetches ciphertext, decrypts, unzips, applies modifications, re-zips, re-encrypts to the current recipient set, uploads.

### 6.3 Reassign

Changing an entry's "Assigned to" list triggers re-encryption:

1. Browser fetches current ciphertext.
2. Browser decrypts with current identity.
3. Browser re-encrypts to the new recipient set.
4. Browser PUTs the updated entry.

For bulk reassignment, the UI batches operations into a single `bulk-reassign` call.

### 6.4 Release UI (`/release`)

The `/release` route reflects the on/off state of inactivity tracking:

- **Off state** (owner has zero entries assigned to a contact): page shows a single explainer card: "Inactivity tracking is off. Assign at least one entry to a designated contact to enable graduated release on extended inactivity." with a CTA to `/vault` or `/contacts`.
- **On state** (owner has ≥1 contact-assigned entry): page shows current state (`ACTIVE`, `REMINDED_SOFT`, etc.), `last_checkin_at`, time until next state transition, configured offsets, and a "Cancel release" affordance during `COOLING` or `FINAL_HOLD`.

The transition between off and on happens implicitly when the owner makes/removes their first contact assignment; no separate enable/disable toggle.

## 7. Anti-patterns

These should not be added:

- A "forgot password" email link. There is no password.
- Storing the user's age private key in localStorage. The identity is per-session-only.
- A "trust this device" shortcut that skips the passkey ceremony.
- Skipping fingerprint verification at contact onboarding.
- Loading fonts or any other resource from a third-party origin.
- A chat widget or any other third-party iframe.
- Using `dangerouslySetInnerHTML`. Render rich text via a sanitizing library if the use case ever genuinely requires it.

## 8. Performance principles

No fixed performance budgets. The one rule: lazy-load `age-encryption` and other heavy crypto code so non-vault routes don't pay for it. Concrete budgets can be added once there is a real bundle to measure.

## 9. Browser bundle integrity

The browser bundle is the highest-risk supply-chain surface. Defenses:

1. **Pinned dependencies.** `package-lock.json` pins every transitive package to an exact version + integrity hash; `npm ci` rejects mismatches.
2. **Audit feeds in CI.** `npm audit --audit-level=high` and `govulncheck` run on every PR; new advisories against pinned versions fail the build.
3. **Bundle SRI.** Vite emits hashed asset filenames; `index.html` is generated at build time with `<script type="module" src="..." integrity="sha384-..."></script>` for every entry. A mismatched bundle fails to load.

`index.html` is served with `Cache-Control: no-cache` (rationale in [Threat Model section 3.2](01-threat-model.md)); hashed assets are served `public, max-age=31536000, immutable`.

## 10. CSP

```
default-src 'self';
script-src 'self';
style-src 'self' 'unsafe-inline';
img-src 'self' data:;
connect-src 'self';
frame-ancestors 'none';
base-uri 'none';
```

## 11. Testing

- **Unit:** Vitest. CryptoSession + age wrapper tested at 100% line coverage.
- **Component:** React Testing Library for every form and critical-flow component.
- **E2E:** Playwright. Critical paths covered:
  - Full registration + first contact invite + first entry.
  - Contact accepts invitation + fingerprint verify (split across two browser contexts).
  - Vault entry CRUD with recipient reassignment.
  - Simulated release: time-fast-forward via test-only API endpoint.
  - False-positive cancel during final hold.
- **Visual:** Storybook for component snapshots.
- **Accessibility:** `@axe-core/playwright` runs on every E2E page; failures block CI.
