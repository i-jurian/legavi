# 07 - Frontend Specification

## 1. Stack

- **Vite + React 19 + TypeScript** (strict mode).
- **Tailwind CSS + shadcn/ui** for styling.
- **TanStack Router** (`@tanstack/react-router`) for routing.
- **TanStack Query** for server-state caching.
- **Zustand** for client state.
- **React Hook Form + Zod** for forms and validation.
- **`@simplewebauthn/browser`** for WebAuthn ceremonies.
- **`@noble/curves`** for X25519 scalar multiplication and Ed25519.
- **`@scure/base`** for bech32 encoding of age keys.
- **`age-encryption`** (npm) for age encryption/decryption.
- **`@zip.js/zip.js`** for browser-side zip/unzip of vault bundles.

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

Authenticated routes use a `beforeLoad` that calls `GET /api/v1/auth/me` and `redirect({ to: "/login" })` on failure.

## 3. Onboarding flow

Invitee opens `/onboarding/:token`, completes WebAuthn registration, sees a fingerprint code to read to the owner over a known-good channel. Owner side: the contact list surfaces unverified contacts with the displayed fingerprint; owner confirms a match or rejects (which triggers re-registration).

## 4. Crypto session lifecycle

Zustand store holding the **32-byte PRF output**. The age keypair is derived on demand via `deriveAgeKeypair(prfBytes)`; only the PRF seed needs zeroing on lock.

### 4.1 Lifecycle states

| State      | Description                                                | Identity in memory? |
| ---------- | ---------------------------------------------------------- | ------------------- |
| `LOCKED`   | No active session, no identity.                            | No                  |
| `UNLOCKED` | Active session, identity available for decrypt operations. | Yes                 |

### 4.2 Lock triggers

All four triggers run the same flow in [lib/session.ts](../frontend/src/lib/session.ts) `lockAndLogout(reason)`: zero PRF bytes, call `POST /api/v1/auth/logout`, redirect to `/login` with a reason-tagged message.

- **Explicit logout:** Sign out on `/dashboard`. No reason message on `/login`.
- **Idle (5 min) and visibility change (60 s):** [hooks/useSessionTimeout.ts](../frontend/src/hooks/useSessionTimeout.ts) mounted in `App.tsx`. Reason `idle` or `hidden`.
- **Server-signaled expiry:** any authenticated API call returning 401 triggers the same flow via the `authFetch` wrapper in [api/auth.ts](../frontend/src/api/auth.ts). Reason `expired`.

PRF bytes are zeroed via `Uint8Array.fill(0)` (best-effort; JS offers no stronger guarantee).

## 5. WebAuthn ceremonies

### 5.1 Registration

```typescript
import { startRegistration } from "@simplewebauthn/browser";
import { registerStart, registerVerify } from "@/api/auth";
import { decodePRFInput, readPRFFirst } from "@/lib/prf";
import { deriveAgeKeypair } from "@/lib/age-keypair";
import { useCryptoSession } from "@/store/cryptoSession";

const { publicKey } = await registerStart({ email, displayName });
decodePRFInput(publicKey);
const response = await startRegistration({ optionsJSON: publicKey });

const prfBytes = readPRFFirst(response);
if (!prfBytes) throw new Error("This passkey does not support PRF.");
const { recipient } = deriveAgeKeypair(prfBytes);

await registerVerify({ ageRecipient: recipient, nickname, response });
useCryptoSession.getState().unlock(prfBytes);
```

`decodePRFInput` patches the base64url PRF salt to a `Uint8Array` in-place because `@simplewebauthn/browser` v13 does not decode PRF inputs before passing them to `navigator.credentials.{create,get}`. Remove when the library handles this upstream.

`deriveAgeKeypair` wraps the raw PRF bytes as the bech32 `AGE-SECRET-KEY-...` identity and computes the `age1...` recipient via `x25519.scalarMultBase`. `@noble/curves` clamps internally.

### 5.2 Authentication

Same shape with `startAuthentication` and `loginStart`/`loginVerify`. PRF output recomputes deterministically; the server already has the recipient from registration so the browser does not resend it.

## 6. Vault UI

### 6.1 Entry list

Sortable, filterable by assignment and label. Each row shows:

- Label hint
- File count
- Assigned to (avatars or names of designated contacts; empty / "-" if owner-only)
- Last updated timestamp
- Quick actions: open, edit, reassign, delete

"Assigned to" lists only designated contacts; the owner is always a recipient but not shown.

Decryption happens lazily: rows display ciphertext metadata until the user clicks to open; clicking decrypts and unzips in memory and shows the bundle's file list.

### 6.2 Create / edit entry

Form: label hint (server-visible), files (drag-drop or picker; 25 MB cap per bundle), assigned-to (multi-select from verified contacts; empty = owner-only).

**Submit:** derive recipient set, zip files via `@zip.js/zip.js`, age-encrypt, POST ciphertext + label + recipient IDs.

**View:** decrypt, unzip, render file list. Inline preview for images/PDFs/text/video/audio via page-scoped blob URLs (revoked on close); other types are download-only.

**Edit:** fetch, decrypt, modify, re-encrypt to the current recipient set, upload.

### 6.3 Reassign

Changing an entry's "Assigned to" list triggers the client-side re-encryption flow in [Crypto Spec section 4.2](02-crypto-spec.md#42-reassignment-semantics). Bulk reassignment batches into a single `bulk-reassign` call.

### 6.4 Release UI (`/release`)

The `/release` route reflects the on/off state of inactivity tracking:

- **Off state** (owner has zero entries assigned to a contact): page shows a single explainer card: "Inactivity tracking is off. Assign at least one entry to a designated contact to enable graduated release on extended inactivity." with a CTA to `/vault` or `/contacts`.
- **On state** (owner has >=1 contact-assigned entry): page shows current state (`ACTIVE`, `REMINDED_SOFT`, etc.), `last_checkin_at`, time until next state transition, configured offsets, and a "Cancel release" affordance during `COOLING` or `FINAL_HOLD`.

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

## 8. Browser bundle integrity

The browser bundle is the highest-risk supply-chain surface. Defenses:

1. **Pinned dependencies.** `package-lock.json` pins every transitive package to an exact version + integrity hash; `npm ci` rejects mismatches.
2. **Audit feeds in CI.** `npm audit --audit-level=high` and `govulncheck` run on every PR; new advisories against pinned versions fail the build.
3. **Bundle SRI.** Vite emits hashed asset filenames; `index.html` is generated at build time with `<script type="module" src="..." integrity="sha384-..."></script>` for every entry. A mismatched bundle fails to load.

`index.html` is served with `Cache-Control: no-cache` (rationale in [Threat Model section 3.2](01-threat-model.md)); hashed assets are served `public, max-age=31536000, immutable`.

## 9. CSP

```
default-src 'self';
script-src 'self';
style-src 'self' 'unsafe-inline';
img-src 'self' data:;
connect-src 'self';
frame-ancestors 'none';
base-uri 'none';
```
