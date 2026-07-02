# DraftRight Infrastructure

Last updated: 2026-07-02

## Production VPS (DigitalOcean, Singapore)

| Item | Value |
|---|---|
| Droplet | `ubuntu-s-1vcpu-2gb-sgp1` — $12 Basic Regular, 2GB / 1 vCPU / 50GB (id 579223373, raw IP `159.223.46.40`) |
| Public entry | **Reserved IP `129.212.208.248`** — GoDaddy A-records point here, never at the raw droplet IP. Droplet swaps = reassign Reserved IP in DO panel, zero DNS edits |
| SSH | `ssh draftright` → `deploy@129.212.208.248` (root login locked; `deploy` has docker group + NOPASSWD sudo) |
| Reverse proxy | Custom xcaddy build v2.11.2 (`maxmind_geolocation` module — stock Caddy fails) at `/usr/bin/caddy`, config `/etc/caddy/Caddyfile`, certs `/var/lib/caddy` |
| History | Migrated 2026-06-21/22 from 4GB/116GB droplet (issue #54). Old box destroyed; manual clean-rebuild method documented in project memory `project_do_droplet_downsize.md` |

Prod and dev stacks are co-located on this one droplet.

## Domains → services

| Domain | Backend |
|---|---|
| `api.draftright.info` | prod Go backend `127.0.0.1:3001` |
| `api.dev.draftright.info` | dev Go backend `127.0.0.1:3101` |
| `draftright.info` | static marketing site `/var/www/draftright` (+ `/downloads/*` installers — verify content-type after any publish, missing files serve 200 HTML fallback) |
| `dev.draftright.info` | dev website build (under `/home/deploy`; caddy user needs `deploy` group to traverse) |
| `admin.draftright.info` / `admin.dev.draftright.info` | admin portal static / `127.0.0.1:5273` |

## Deploy layout on the VPS

| Path | Purpose |
|---|---|
| `/opt/draftright/` | prod runtime: `docker-compose.prod.yml` + `.env` (root-owned — edit via sudo, back up first) |
| `/home/deploy/deploys/draftright-main/` | git checkout of `main` — prod image builds |
| `/home/deploy/deploys/draftright-dev/` | git checkout of `develop` + `.env.dev` — dev stack |
| `/var/lib/draftright/bug-reports` | uploaded screenshots (bind-mounted into backend) |
| `/var/www/` | static sites (draftright marketing incl `/downloads`, admin, southernmartin) |

## Containers (Go backend era — Node decommissioned)

Prod: `draftright-backend-go-1` (:3001), `draftright-postgres-1` (pg16, db `draftright`), `draftright-redis-1`.
Dev: `dr-backend-go-dev` (:3101), `draftright-dev-postgres-1` (db `draftright_dev`), `draftright-dev-redis-1`, `dr-admin-dev` (:5273).

## Redeploy recipes

```bash
# PROD backend (from main)
ssh draftright
cd /home/deploy/deploys/draftright-main && git fetch origin && git merge --ff-only origin/main
docker build -t draftright-backend-go:latest backend-rewrite-go
cd /opt/draftright && docker compose -f docker-compose.prod.yml up -d backend-go

# DEV backend (from develop)
cd /home/deploy/deploys/draftright-dev && git fetch origin && git merge --ff-only origin/develop
docker compose --env-file .env.dev -f docker-compose.yml -f docker-compose.dev.yml up -d --build backend-go-dev
```

Post-deploy: `docker ps` (all healthy) + `curl https://api.draftright.info/health`.

## Environment gotchas

- **Go backend ignores `NODE_ENV` — prod mode needs `APP_ENV=production`** (set in compose; issue #46).
- **Streaming `/v1/rewrite` reads `OPENAI_API_KEY` from env, NOT from the `ai_providers` table.** Compose pins `AI_PROVIDERS=openai` + `OPENAI_PROVIDER_ID` (the row used for `usage_logs` FK), but the actual key comes from `/opt/draftright/.env`. If env key and the DB row drift, streaming 401s silently while the parity `/rewrite` path (DB-keyed) keeps working. Bit prod until 2026-07-02 — the env held the Ollama Cloud key.
- Dev `.env.dev` has a standalone `DATABASE_URL` whose password can drift from `POSTGRES_PASSWORD` — fresh pg volumes init from the latter; align with `ALTER USER` if the dev backend crash-loops on `28P01`.
- Caddy `request_body max_size 6MB` on both API vhosts (bug-report screenshots; a proxy 413 without CORS headers masquerades as a CORS error).

## Databases

- PostgreSQL 16 (alpine) in Docker, user `draftright`.
- Prod db `draftright`; dev db `draftright_dev` (dev container also carries an unused `draftright` db).
- Key tables: `users`, `admin_users`, `plans`, `subscriptions`, `ai_providers`, `app_settings`, `usage_logs`, `rewrite_logs` (training data), `bug_reports`, `feature_votes`, `payments`.
- Migrations are plain SQL under `backend/sql/` — apply on prod before rebuilding containers.

## Local Development

| Service | How | Port |
|---|---|---|
| PostgreSQL | `docker compose up -d postgres` | 5432 |
| Backend API (Node, parity authority) | `cd backend && npm run start:dev` | 3000 |
| Go backend | `cd backend-rewrite-go && go run ./cmd/server` | per `LISTEN_ADDR` |
| Admin Portal | `cd admin && npm run dev` | 5173 |
| Website | `cd website && npm run dev` | 4000 |

## Test Devices

- Samsung A52 (Android) — sideloaded 2.4.x builds via ADB
- iPhone (real device) — keyboard extension can't run in simulator (#14)
- macOS dev machine — V1 + V2 coexist
