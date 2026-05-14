# 00 - Product Requirements

## 1. Problem

People accumulate digital secrets that matter to others after their death or extended incapacity: account credentials, financial logins, photo archives, business documents, instructions for survivors. Existing solutions are operator-trusted, vendor-locked, or so brittle they fail silently. The owner needs a way to:

- Store these secrets securely while alive.
- Assign each piece of data to a specific designated recipient.
- Have the system deliver the data when the owner is provably inactive, neither earlier nor not at all.
- Trust that the operator (and any other non-designated party) cannot read the data, ever.

## 2. Goals

1. Owner can store, edit, and access their vault while alive, with full confidentiality from the operator.
2. Owner can designate up to 10 contacts and assign each vault entry to a recipient (or set of recipients), or keep it owner-only.
3. On owner inactivity, the system orchestrates a graduated release process with reminders, cooling-off periods, and a final-hold cancel window.
4. After release fires, each recipient independently receives only the data the owner assigned to them. No coordination between recipients required.
5. Compromise of any single recipient affects only their assigned data; the rest of the vault remains protected.
6. The system is self-hostable. Self-hosters control their own infrastructure and trust the same code their users see.
7. Owner can complete onboarding in under 30 minutes on a clean device.
8. A non-technical contact can complete invitation acceptance in under 10 minutes.

## 3. Out of scope

- Real-time messaging or chat.
- Cryptocurrency wallet management.
- Mobile-native apps. Browser is the only client.
- Older browsers without WebAuthn support.
- SMS for liveness or delivery.
- Legal will substitution or estate-planning service integration. The system is a complement to a legal will, not a replacement.
- Multi-vault management within a single owner account.
- Group ownership (multiple owners on one vault).
- Threshold release. No T-of-N recipient cooperation mode; each entry has explicit recipient(s) and any one of them decrypts independently.
- Recovery without designated recipients. Owner-only entries are unrecoverable if the owner loses their passkey and has no passphrase backup export.

## 4. Functional requirements

- Passkey-based registration. WebAuthn credentials with user verification. No passwords.
- Vault entries encrypted on the user's device. Each entry is a zip bundle of one or more files (zipped client-side, then age-encrypted; default cap: 25 MB per bundle, configurable). The owner's age identity (derived from their passkey's PRF extension) is included as a recipient on every entry.
- Up to 10 contacts, invited by email, each registering their own passkey.
- Per-entry recipient assignment. Each vault entry is age-encrypted to the owner plus zero or more designated contacts (any assigned recipient can decrypt independently). Owner-only entries never enter the release flow; entries with at least one contact do. The owner assigns or reassigns recipients at any time while alive.
- Out-of-band fingerprint verification at contact onboarding.
- Inactivity detection via owner check-in timestamp.
- Graduated reminder cadence (configurable; defaults: 7d soft, 14d firm, 30d final).
- 48-hour firm cooling period before release fires, during which the owner can cancel.
- 24-hour final hold with a false-positive cancel flag.
- Hash-chained audit log with periodic Ed25519 signed checkpoints, visible to the owner.
- Encrypted backup export, decryptable by the owner with their passkey or with a passphrase the owner sets at export time.

## 5. Legal context

Not legal advice.

- **Lawful intercept.** A court order against the operator yields only ciphertext and metadata; the operator holds no plaintext. A court order against the owner can be complied with (they hold the passkey). A recipient can comply for the data assigned to them only ([Threat Model 3.6](01-threat-model.md)).
- **Vendor lock-in.** Self-hosters can fork. Hosted-instance users should download encrypted backup exports periodically ([Crypto Spec 8](02-crypto-spec.md)).
