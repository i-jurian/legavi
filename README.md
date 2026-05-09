# Legavi

Encrypted file vault with optional inactivity-triggered release. Files can be kept owner-only or assigned to designated recipients; assigned files are released on extended owner inactivity.

## How it works

The owner signs in with the device's biometric or a hardware security key. No passwords. Vault contents are encrypted on-device; the server stores only ciphertext. The owner assigns each piece of data to a designated recipient. If the owner stops checking in, the system notifies each recipient. Each recipient signs in with their own biometric and receives only what was assigned to them.

## What this project is

1. **Per-recipient release** - each piece of data goes only to the designated recipient.
2. **Passkey-native authentication** - no passwords, no password resets, hardware-bound credentials.
3. **End-to-end encryption** - keys derived on-device, ciphertext only on the server.
4. **Self-hostable** - runs on chosen infrastructure. Third-party trust optional.
5. **Graduated release** - multi-stage warnings, cooling-off period, owner-cancel window.

## Status

See [implementation status](docs/IMPLEMENTATION_STATUS.md).

## Quick links

- [Product Requirements](docs/00-prd.md)
- [Threat Model](docs/01-threat-model.md)
- [Cryptographic Specification](docs/02-crypto-spec.md)
- [System Architecture](docs/03-architecture.md)
- [Milestones](docs/10-milestones.md)

## Repository layout

```
legavi/
├── README.md
├── LICENSE              # Apache 2.0
├── docs/                # Specifications
├── backend/             # Go HTTP API, scheduler, worker (stdlib + chi)
├── frontend/            # React + TypeScript SPA, WebAuthn ceremonies, age decryption
├── deploy/
│   └── docker/          # Docker Compose for dev/local
└── .github/             # CI, issue/PR templates
```

## Getting started (development)

Prerequisites: Docker, Docker Compose, Go 1.22+, Node 20+, Make.

```bash
git clone https://github.com/i-jurian/legavi.git
cd legavi
make dev
open http://localhost:5173
```

### Commands

| Command         | What it does                              |
| --------------- | ----------------------------------------- |
| `make dev`      | Spin up Postgres, MailHog, Caddy, backend |
| `make dev-down` | Stop the dev stack                        |
| `make test`     | Run backend + frontend tests              |
| `make lint`     | Run backend + frontend linters            |

See [Deployment Guide](docs/09-deployment.md) for production.

## License

Apache 2.0. See [LICENSE](LICENSE).
