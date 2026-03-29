# DraftRight

AI-powered text rewriting platform — select text, pick a tone, get a polished rewrite.

## Quick Facts

| Item | Value |
|---|---|
| Owner | Tan Nguyen |
| Versions | V1 (standalone, tag `v1.0`) / V2 (backend-powered, `main`) |
| Platforms | macOS, Android, iOS, Windows, Linux, Web Admin |

## Architecture

```
DraftRight/          # macOS native app (Swift/SwiftUI)
DraftRightMobile/    # Flutter app + iOS/Android keyboard extensions + desktop
backend/             # NestJS API + PostgreSQL
admin/               # React admin portal (Tailwind, Vite)
docker-compose.yml   # Backend + Postgres + Admin
docs/                # Specs, plans, superpowers
```

## Tech Stack

| Component | Stack |
|---|---|
| macOS app | Swift 5.9, SwiftUI, AppKit, macOS 13+ |
| Mobile app | Flutter 3.x, Dart |
| Android keyboard | Kotlin, InputMethodService |
| iOS keyboard | Swift, UIInputViewController |
| Desktop (Win/Linux) | Flutter Desktop, system_tray, hotkey_manager |
| Backend API | NestJS 10+, TypeScript, TypeORM, PostgreSQL 16 |
| Admin portal | React 18, Vite, Tailwind CSS (Modernize dark theme) |

## Quick Start

```bash
# Start backend + database
docker compose up -d postgres
cd backend && cp .env.example .env  # edit with real values
source .env && npx ts-node src/seed.ts
npm run start:dev                    # http://localhost:3000

# Start admin portal
cd admin && npm run dev              # http://localhost:5173

# Swagger docs
open http://localhost:3000/api/docs
```

## Git Workflow

Standard GitFlow — see `~/.claude/CLAUDE.md` for full rules.
- Branch from `develop`: `feature/<description>-<YYYYMMDD>`
- Commit prefixes: `feat:`, `fix:`, `chore:`, `docs:`
- Never commit directly to `main` or `develop`
- `DraftRightMobile/` is a git submodule — commit inside it separately

## Versions

- **V1** (`v1.0` tag) — standalone, users provide their own OpenAI API key, no backend
- **V2** (`main`) — backend-powered with auth, subscriptions, usage limits, admin portal
- Both install side-by-side (different bundle IDs: `*.v2` suffix)

## Key Ports

| Service | Port |
|---|---|
| Backend API | 3000 |
| PostgreSQL | 5432 |
| Admin Portal | 5173 (dev) / 3001 (Docker) |
| Swagger Docs | 3000/api/docs |

## Admin Credentials (dev)

- Email: `admin@draftright.com`
- Password: `DraftRight2026`

## Subdirectory Docs

- `backend/CLAUDE.md` — API modules, database, auth
- `admin/CLAUDE.md` — Admin portal pages, API client
- `DraftRightMobile/CLAUDE.md` — Flutter app, keyboard extensions
- `DraftRight/CLAUDE.md` — macOS native app
