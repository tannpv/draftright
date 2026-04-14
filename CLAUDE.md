# DraftRight

AI-powered text rewriting platform — select text, pick a tone, get a polished rewrite.

## Quick Facts

| Item | Value |
|---|---|
| Owner | Tan Nguyen |
| Versions | V1 (standalone, tag `v1.0`) / V2 (backend-powered, `main`) |
| Platforms | macOS, iOS, Android, Windows, Linux, Web |

## Architecture

```
DraftRight/            # macOS native app (Swift/SwiftUI)
DraftRightMobile/      # Flutter app + iOS/Android keyboard & share extensions
DraftRightWindows/     # Windows native app (WinUI 3 / C# / .NET 8)
DraftRightLinux/       # Linux native app (GTK4 / libadwaita / Python)
backend/               # NestJS API + PostgreSQL + Redis
admin/                 # React admin portal (Tailwind, Vite)
website/               # Astro marketing site + web playground
docker-compose.yml     # Backend + Postgres + Redis + Ollama + Website
docs/                  # Specs, plans (Windows & Linux native app plans)
```

## Tech Stack

| Component | Stack |
|---|---|
| macOS app | Swift 5.9, SwiftUI, AppKit, macOS 13+ |
| iOS app | Flutter 3.x + Swift keyboard/share extensions |
| Android app | Flutter 3.x + Kotlin keyboard extension |
| Windows app | WinUI 3, C# 12, .NET 8, MSIX |
| Linux app | GTK4, libadwaita, Python 3.11+ |
| Backend API | NestJS 10+, TypeScript, TypeORM, PostgreSQL 16, Redis |
| Admin portal | React 18, Vite, Tailwind CSS (Modernize dark theme) |
| Marketing site | Astro 5, React 18 (islands), Tailwind CSS |
| AI providers | OpenAI, Anthropic (Claude), Ollama (local), Custom |

## Quick Start

```bash
# Start infrastructure
docker compose up -d postgres redis

# Start backend
cd backend && cp .env.example .env  # edit with real values
ADMIN_PASSWORD=DraftRight2026 npx ts-node src/seed.ts
npm run start:dev                    # http://localhost:3000

# Start admin portal
cd admin && npm run dev              # http://localhost:5173

# Start marketing website
cd website && npm run dev            # http://localhost:4000

# Start Ollama (local AI)
open /Applications/Ollama.app        # or docker compose up -d ollama
ollama pull llama3.2
```

## Key Ports

| Service | Port |
|---|---|
| Backend API | 3000 |
| Admin Portal | 5173 (dev) |
| Marketing Website | 4000 (dev) |
| PostgreSQL | 5432 |
| Redis | 6379 |
| Ollama | 11434 |
| Swagger Docs | 3000/api/docs |

## Admin Credentials (dev)

- Email: `admin@draftright.com`
- Password: `DraftRight2026`
- **Login at:** `/admin/auth/login` (separate from customer auth)

## Auth Separation

| Table | Users | Auth Endpoint |
|---|---|---|
| `admin_users` | Portal admins | `POST /admin/auth/login` |
| `users` | Customers | `POST /auth/login` |

## AI Providers

Default: Ollama Llama 3.2 (free, local). Switchable in Admin > AI Providers.
Supports: OpenAI, Anthropic (Claude), Ollama, any OpenAI-compatible API.

## Payment Methods

Stripe, PayPal, Momo, VietQR (MB Bank), Bank Transfer. Configure in Admin > Settings > Payment.

## Git Workflow

Standard GitFlow — see `~/.claude/CLAUDE.md` for full rules.
- Branch from `develop`: `feature/<description>-<YYYYMMDD>`
- Commit prefixes: `feat:`, `fix:`, `chore:`, `docs:`
- Never commit directly to `main` or `develop`
- `DraftRightMobile/` is a git submodule — commit inside it separately

## Subdirectory Docs

- `backend/CLAUDE.md` — API modules, database, auth
- `admin/CLAUDE.md` — Admin portal pages, API client
- `DraftRightMobile/CLAUDE.md` — Flutter app, keyboard extensions
- `DraftRight/CLAUDE.md` — macOS native app
- `website/CLAUDE.md` — Marketing site, web playground
- `DraftRightLinux/CLAUDE.md` — Linux native app
- `docs/superpowers/plans/` — Implementation plans for Windows & Linux native apps
