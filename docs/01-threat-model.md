# 01 - Threat Model

**Update history**

- 2026-05-09: Initial draft

---

## 1. Trust diagram

| Party                                  | Trust level                    | What they know                                                                                     |
| -------------------------------------- | ------------------------------ | -------------------------------------------------------------------------------------------------- |
| Owner                                  | FULL TRUST                     | Their passkey, their vault contents                                                                |
| Owner's device                         | FULL TRUST                     | Passkey material in hardware-backed keystore (Secure Enclave, TPM, FIDO2 chip)                     |
| Operator                               | ZERO-KNOWLEDGE for owner data  | Ciphertext, public credentials, audit log entries, scheduling state, recipient-assignment metadata |
| Server infrastructure                  | ZERO-KNOWLEDGE for owner data  | Same as operator                                                                                   |
| Designated recipient                   | TRUSTED for assigned data only | Their own passkey, the data the owner explicitly assigned to them after release fires              |
| Recipient's device                     | TRUSTED for assigned data only | Recipient's passkey material in hardware-backed keystore                                           |
| Passkey sync provider                  | OPTIONAL TRUST                 | Encrypted passkey blobs if owner or recipient opts into sync                                       |

The owner assigns each piece of data to a specific recipient. The system enforces that no one outside the assigned recipient set can read the data; recipient choice is the owner's. Operator compromise leaks ciphertext and metadata, never plaintext.

## 2. Asset table

| Asset                                                                           | Where it lives                                                                 |
| ------------------------------------------------------------------------------- | ------------------------------------------------------------------------------ |
| Owner's vault entries (plaintext)                                               | Only in the owner's browser memory while logged in                             |
| Owner's age identity (private X25519 key)                                       | Derived just-in-time from passkey PRF; never persisted                         |
| Vault entries (ciphertext)                                                      | Server, each entry age-encrypted to the owner and to its assigned recipient(s) |
| Owner's passkey                                                                 | Owner's device hardware (Secure Enclave / TPM / FIDO2 chip)                    |
| Recipient's passkey                                                             | Recipient's device hardware                                                    |
| Recipient's age identity (private X25519 key)                                   | Derived just-in-time from recipient's passkey PRF                              |
| Recipient-assignment metadata (which entry has which recipient)                 | Server, plaintext (visible to operator); the data itself remains ciphertext    |
| Audit log entries                                                               | Server, hash-chained, with periodic Ed25519 signed checkpoints                 |
| Audit log signing key (Ed25519 private)                                         | Owner's device, derived from passkey PRF with a domain-separation tag          |
| Release state                                                                   | Server (last check-in, current state, etc.)                                    |
| Recipient public credentials (passkey credential ID, public key, age recipient) | Server                                                                         |
| Recipient email addresses                                                       | Server                                                                         |

## 3. Adversaries and threat scenarios

### 3.1 Curious operator

The operator wants to read the owner's vault.

**What they have:** ciphertext, public keys, audit log, recipient-assignment metadata, scheduling state.

**What they cannot do:**

- Read vault entries. Plaintext lives only on the owner's or designated recipient's device after a successful authentication.
- Forge a signed audit log entry. Audit signing key lives on the owner's device.
- Decrypt entries on their own. The age recipients on every entry are the owner and the assigned recipient(s). The operator is never a recipient.

**What they can do:**

- See metadata: who has accounts, when owners check in, who their recipients are, what each recipient is assigned to receive (entry IDs and labels, not contents), when entries are retrieved post-release.
- Selectively withhold or rewrite audit log content (caught by signed checkpoint mismatch).

**Mitigations:**

- Audit log uses hash-chained entries with periodic owner-signed checkpoints. Owner's browser verifies the chain root on each session.
- Owner is informed at onboarding that connection metadata is visible to the operator.
- Self-hosting is documented as the recommended deployment for high-assurance threat models.

### 3.2 Malicious operator (push code)

A determined operator can push modified frontend code that exfiltrates passkey-derived material at the moment of authentication.

**What they cannot do without code modification:**

- Read past vault contents from the database alone.

**What they can do with code modification:**

- Phish active users by serving malicious JS at authentication time.

**Mitigations:**

- Subresource Integrity (SRI) on the deployed bundle so a modified asset fails to load.
- Cache-Control: no-cache on `index.html` so a CSP-violating bundle replacement is short-lived.
- Reproducible bundles would let a self-hoster verify that the deployed bundle matches the open-source code (deferred).

### 3.3 Server compromise

External attacker gains unauthorized access to the server (RCE, SQL injection, leaked credentials). Outcome and mitigations are the same as the malicious-operator case (3.2). Additional baseline: dependency scanning in CI, minimal attack surface, structured logging that excludes secret material.

### 3.4 Stolen owner device

Attacker has physical possession of the owner's primary device.

**What they cannot do:**

- Use the passkey without biometric or device PIN. WebAuthn user verification is hardware-enforced.
- Export the passkey from the hardware keystore.

**What they can do:**

- If the device session is unlocked AND the user is logged into the vault, they can read entries.
- Attempt to bypass biometric (high-effort).

**Mitigations:**

- Vault locks after 5 minutes of inactivity (CryptoSession idle timeout).
- Vault locks on tab visibility change (after 60 seconds).
- Biometric required at every login ceremony.
- Owner can revoke a device's passkey from another logged-in device.

### 3.5 Stolen recipient device

Attacker has physical possession of a designated recipient's device.

**What they cannot do:**

- Use the recipient's passkey without biometric or device PIN.

**What they can do:**

- If they bypass biometric AND release has fired, they can decrypt the data the owner assigned to that recipient.
- They cannot decrypt data assigned to other recipients.

**Mitigations:**

- Owner is encouraged to assign data thoughtfully (don't give the highest-stakes credentials to the recipient with the weakest device-security posture).

### 3.6 Coerced or compromised recipient

A designated recipient betrays trust - either willingly, under social pressure, or via legal compulsion.

**What they can do:**

- Recover the data the owner assigned to them, after release fires. The system does not prevent this.

**What they cannot do:**

- Recover data assigned to other recipients.
- Recover anything before release fires (their access is gated by the release state machine on the server side).

Recipient vetting is the owner's responsibility.

**Mitigations:**

- Out-of-band fingerprint verification at onboarding catches MITM attacks during initial recipient setup.
- The release state machine (cooling period, final hold) gives the owner time to detect early-release attempts and cancel.

### 3.7 Owner makes a wrong-way release decision

False positive: the owner is alive but the system fires a release because the owner missed reminders.

**Mitigations:**

- Graduated reminders escalate over weeks, not days.
- Configurable cadence per owner.
- Final hold provides 24 hours of cancellation window after the formal trigger fires.
- False-positive flag during the final hold revokes recipient access tokens before they're consumed.

### 3.8 Compromised passkey sync provider

Owner or recipient opted into platform passkey sync. The sync provider is breached.

**What they get:**

- The synced passkey blob. If they can decrypt it, they have a usable passkey copy.

**Mitigations:**

- Hardware FIDO2 keys (no sync) for users who want maximum protection.
- On the recipient side, compartmentalization limits damage to that recipient's assigned data.
- On the owner side, no technical mitigation: a synced-key compromise lets the attacker authenticate as the owner.
- Documented in onboarding so the owner can choose synced (better UX) versus hardware-only (better security).

### 3.9 Solo-operator threat

In a solo self-hosted deployment, the developer is also the operator. Mitigations that depend on operator-vs-owner separation degrade in that setup.

**Mitigations:**

- Audit log replication off-server (recommended where feasible).
- Solo self-hosting is documented as a weaker variant of the threat model, not as fully operator-blind.

## 4. Out-of-scope threats

The system does not protect against:

- **Quantum-capable adversaries.** X25519 and Ed25519 are classical-secure only.
- **Compromise of the owner's primary device while logged in.** An attacker with unlocked-and-logged-in access reads what the owner can read.
- **Coercion of designated recipients for the data assigned to them.** See 3.6.
- **Targeted exploitation of a zero-day in the browser, OS, hardware authenticator, or platform sync infrastructure.** This system relies on those components being uncompromised; if they're not, the security model fails. Specific zero-days are not addressed.
- **Legal compulsion against the owner directly.** A court order requiring the owner to authenticate cannot be technically resisted; the owner is the trust root.

## 5. Summary of protections

| Layer                | Defense                                                                                                                             |
| -------------------- | ----------------------------------------------------------------------------------------------------------------------------------- |
| Authentication       | Passkey with hardware user verification. Phishing-resistant by domain binding.                                                      |
| Owner device         | Vault locks idle and on visibility change. Passkey hardware-bound.                                                                  |
| Network              | TLS 1.3, HSTS, strict CSP, SRI on bundle.                                                                                           |
| Server               | Ciphertext only. Public credentials only. Hash-chained audit log.                                                                   |
| Compartmentalization | Each entry encrypted only to its designated recipient(s) plus the owner. One recipient compromise affects only their assigned data. |
| Release process      | Graduated cadence, cooling periods, final-hold cancel.                                                                              |
| Audit                | Owner-signed checkpoints in audit log. Off-server replication recommended for high-assurance deployments.                           |
