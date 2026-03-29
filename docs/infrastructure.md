# DraftRight Infrastructure

## Local Development

| Service | How | Port |
|---|---|---|
| PostgreSQL | Docker Compose (`docker compose up -d postgres`) | 5432 |
| Backend API | `cd backend && npm run start:dev` | 3000 |
| Admin Portal | `cd admin && npm run dev` | 5173 |
| Swagger Docs | Auto at backend `/api/docs` | 3000 |

## Docker Compose Services

- `postgres` — PostgreSQL 16 Alpine, volume `pgdata`
- `backend` — NestJS app, depends on postgres healthy
- `admin` — nginx serving React static files (port 3001)

## Database

- PostgreSQL 16
- User: `draftright`, Password: `password`, DB: `draftright`
- Tables: `users`, `plans`, `subscriptions`, `ai_providers`, `usage_logs`
- TypeORM with `synchronize: true` in dev

## Production (TBD)

- Not yet deployed
- Plan: VPS with Docker Compose, nginx reverse proxy, Let's Encrypt SSL
- Domain: TBD

## Test Devices

- Samsung A52 (Android) — has V1 installed, V2 via wireless ADB
- macOS (dev machine) — both V1 and V2 coexist
