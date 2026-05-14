# 09 - Deployment Guide

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

- Postgres 18 on `localhost:5432` (Docker Compose, `deploy/docker/compose.yaml`)
- MailHog on `localhost:1025` (SMTP) and `localhost:8025` (UI)
- Backend on `localhost:8080` via `go run ./cmd/api` (no auto-reload)
- Frontend on `localhost:5173` (Vite HMR)

Open `http://localhost:5173`; the frontend proxies `/api/*` to `:8080`. The compose file's Caddy entry is commented out.

```bash
make dev-down   # stop Docker services; Ctrl-C the foreground processes
```

## 3. Configuration

For development, defaults in `deploy/docker/.env.example` work:

```bash
cp deploy/docker/.env.example deploy/docker/.env
# edit if needed
```

All variables in [Architecture section 7](03-architecture.md#7-configuration) are required at startup. Deployment-relevant summary:

| Variable              | Example                                  | Purpose                                                                  |
| --------------------- | ---------------------------------------- | ------------------------------------------------------------------------ |
| `LGV_PUBLIC_URL`      | `https://vault.example.com`              | Public URL; WebAuthn RP ID is its hostname                               |
| `LGV_DATABASE_URL`    | `postgres://user:pass@db:5432/legavi`    | Postgres connection string                                               |
| `LGV_JWT_SIGNING_KEY` | `openssl rand -base64 32`                | Session JWT key; decodes to >= 32 bytes                                  |
| `LGV_JWT_TTL`         | `24h`                                    | `lgv_session` lifetime; any `time.ParseDuration` value                   |
| `LGV_API_LISTEN`      | `:8080`                                  | Bind address                                                             |
| `LGV_SMTP_URL`        | `smtp://user:pass@smtp.example.com:587`  | SMTP relay                                                               |
| `LGV_FROM_EMAIL`      | `noreply@vault.example.com`              | From address; configure SPF / DKIM / DMARC for the sending domain        |
| `LGV_LOG_LEVEL`       | `info`                                   | `debug`, `info`, `warn`, `error`                                         |
| `LGV_TEST_MODE`       | `false`                                  | Must be `false` in production                                            |


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
