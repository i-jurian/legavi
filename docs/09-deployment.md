# 09 - Deployment Guide

**Update history**

- 2026-05-09: Initial draft

---

Development and production deployment. The MVP target is solo self-hosted via Docker Compose.

## 1. Prerequisites

| Tool           | Minimum version     |
| -------------- | ------------------- |
| Docker         | 24.0                |
| Docker Compose | v2                  |
| Go             | 1.22                |
| Node           | 20 (24 recommended) |
| Make           | any                 |
| `git`          | any                 |

## 2. Development setup

```bash
git clone https://github.com/<owner>/legavi.git
cd legavi
make dev
```

`make dev` brings up:

- Postgres 16 on `localhost:5432`
- MailHog (SMTP catch-all) on `localhost:1025` (SMTP) and `localhost:8025` (web UI)
- Caddy reverse proxy on `localhost:8080`
- Backend API server (rebuilt on file changes)
- Frontend dev server (Vite HMR) on `localhost:5173`

Access the app at `http://localhost:8080` (proxied) or `http://localhost:5173` (direct frontend with API proxy).

Stop:

```bash
make dev-down
```

## 3. Configuration

All configuration is environment-variable based ([Architecture section 7](03-architecture.md)). For development, defaults in `deploy/docker/.env.example` are sufficient. Copy to `.env`:

```bash
cp deploy/docker/.env.example deploy/docker/.env
# edit if needed
```

Required for production deployment (no defaults):

| Variable              | Purpose                                                                                                                              |
| --------------------- | ------------------------------------------------------------------------------------------------------------------------------------ |
| `LGV_PUBLIC_URL`      | Full https URL of the public deployment (e.g., `https://vault.example.com`)                                                          |
| `LGV_DATABASE_URL`    | Postgres connection string                                                                                                           |
| `LGV_JWT_SIGNING_KEY` | 32+ random bytes, base64                                                                                                             |
| `LGV_SMTP_URL`        | SMTP relay URL (e.g., `smtp://user:pass@smtp.example.com:587`)                                                                       |
| `LGV_FROM_EMAIL`      | "From" address on outbound mail. Configure SPF, DKIM, and DMARC for the sending domain; release notifications must reach recipients. |

Generate a JWT signing key:

```bash
openssl rand -base64 32
```

## 4. Production topology (single-host)

The simplest production deployment runs everything on a single VM:

```mermaid
flowchart TB
    Internet([Internet]) -->|:443| Caddy[Caddy<br/>reverse proxy + TLS]

    subgraph Backend["Backend"]
        API[api]
        Scheduler[scheduler]
        Worker[worker]
    end

    Caddy --> API

    API <--> DB[(Postgres)]
    Scheduler <--> DB
    Worker <--> DB
    Worker -.->|SMTP :587| SMTP([SMTP relay])
```

All four app processes plus Postgres run via `docker compose -f deploy/docker/compose.prod.yaml up -d` on a single host. Caddy automatically obtains a TLS certificate via Let's Encrypt for the configured `LGV_PUBLIC_URL`.

System requirements:

- 2 vCPU, 4 GB RAM minimum.
- 20 GB disk for app + 50 GB for Postgres growth (depends on user count and audit log retention).
- Static IP or domain name with A/AAAA records pointing at the host.
- Ports 80 and 443 reachable from the internet; all other ports closed (Postgres bound to `127.0.0.1` in compose.prod.yaml, never exposed).

## 5. Production deployment steps

```bash
# On the production host:
git clone https://github.com/<owner>/legavi.git
cd legavi
cp deploy/docker/.env.example deploy/docker/.env
# Edit .env with your production values
docker compose -f deploy/docker/compose.prod.yaml up -d
docker compose -f deploy/docker/compose.prod.yaml logs -f
```

The API server runs migrations automatically on startup. After the first deployment, verify:

```bash
curl -fsS https://your-domain.example/healthz
curl -fsS https://your-domain.example/readyz
```

Both should return `200 OK`.

## 6. Backups

Production must run nightly Postgres backups. The reference setup:

```bash
# In a cron job on the host:
docker compose -f deploy/docker/compose.prod.yaml exec -T postgres \
  pg_dump -U lv lv_production | \
  gzip > /var/backups/lv-$(date +%Y%m%d).sql.gz
```

Backups should be encrypted at rest and shipped offsite.

Restore to a scratch instance monthly to verify integrity:

```bash
gunzip -c /var/backups/lv-YYYYMMDD.sql.gz | \
  docker compose -f deploy/docker/compose.scratch.yaml exec -T postgres \
  psql -U lv lv_scratch
```

## 7. Updates

```bash
cd legavi
git fetch && git pull
docker compose -f deploy/docker/compose.prod.yaml build
docker compose -f deploy/docker/compose.prod.yaml up -d
```

Database migrations apply automatically on backend startup. Watch the logs to confirm the migration succeeded before declaring the update complete.

If a deploy fails: `git checkout` the prior commit, restore the latest backup if the migration corrupted state, and `docker compose ... up -d --build` to redeploy the previous version.

For breaking schema changes (rare), the release notes will call out manual steps; read the changelog before updating production.

To rotate `LGV_JWT_SIGNING_KEY`: replace the env value and restart. All existing sessions are invalidated; users re-authenticate via passkey on next request.

## 8. TLS

Caddy handles TLS automatically via Let's Encrypt. Requirements:

- The host's IP must match the DNS A/AAAA record for `LGV_PUBLIC_URL`.
- Ports 80 and 443 must be reachable from the public internet (Let's Encrypt's HTTP-01 challenge).

For air-gapped or self-issued certificates, edit `deploy/docker/Caddyfile` to use a custom certificate source.

HSTS is enabled with a 1-year max-age once a valid certificate is in use. Rolling back to HTTP requires waiting out the cached max-age in every browser that visited the site.
