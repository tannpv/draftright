# DraftRight

AI-powered text rewriting platform — select text, pick a tone, get a polished rewrite.

## RULE #1 — Clean, Reusable, Extendable

**Standing first rule, applied to every task: new, in-progress, and already-merged-but-unfinished.**

- **Clean** — small focused units, clear names, no dead code; comments say *why*.
- **Reusable** — extract shared logic; never copy-paste. One source of truth per fact.
- **Extendable** — design for the next platform, caller, or provider.
- **No hardcoding** — any value that carries meaning is an enum/const/config with
  ONE source of truth, never a literal at a call site. Applies to domain values,
  platform names, tuning numbers, colours, URLs, column lists, and SQL.
  **Duplicated logic counts** — two copies are two sources of truth and they will
  drift. If two copies genuinely can't be merged (different languages/transports),
  add a test that asserts they agree.

No exemptions for "small change", "existing code does it this way", or "that part
already merged". When resuming a task, audit what already landed before adding to it.

Not a style preference: issue **#22** is the worked example — a copy-pasted
`app_releases` upsert drifted between the manual and CI release paths, and
Windows shipped unverified installers to production for two months. Read that
issue before arguing a duplicate is harmless.

> This section is a **summary for contributors**, kept self-contained because a
> fresh clone has no other copy. The canonical rule, full checklist, and
> no-hardcoding detail live in the maintainer's `~/.claude/CLAUDE.md`; if the two
> disagree, that one wins. Do not expand this section — extend the canonical one.

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

## Before Merging — Every Task

After implementing and before `--no-ff` merging to develop, two steps are
**mandatory** on every task — new, in progress, merged-but-unfinished, and any
plan still in the backlog. A plan that doesn't end in both is not finished.

1. **Clean garbage** — `/cleanup-garbage`. Delete what the change orphaned: dead
   code, unused DB tables/columns, stale config and env vars, unused deps,
   leftover files. **DELETE, never deprecate** — no commented-out code, no
   `// TODO remove`. DB drops need a reversible migration + backup, never a
   hand-drop on prod.
2. **Full review** — `/epiphanydev:full-review` over the diff: correctness,
   RULE #1 compliance, security. Fix findings before merging.

Full checklist (19 steps, test-cases-first through issue closure) lives in the
maintainer's `~/.claude/CLAUDE.md`.

## Git Workflow

Standard GitFlow — see `~/.claude/CLAUDE.md` for full rules.
- Branch from `develop`: `feature/<description>-<YYYYMMDD>`
- Commit prefixes: `feat:`, `fix:`, `chore:`, `docs:`
- Never commit directly to `main` or `develop`
- `DraftRightMobile/` is a plain directory in the monorepo (absorbed from the standalone repo via `git subtree` 2026-05-11). To pull future upstream changes, use `git subtree pull --prefix=DraftRightMobile draftrightmobile main`. The `draftrightmobile` remote stays configured for that purpose.

## Subdirectory Docs

- `backend/CLAUDE.md` — API modules, database, auth
- `admin/CLAUDE.md` — Admin portal pages, API client
- `DraftRightMobile/CLAUDE.md` — Flutter app, keyboard extensions
- `DraftRight/CLAUDE.md` — macOS native app
- `website/CLAUDE.md` — Marketing site, web playground
- `DraftRightLinux/CLAUDE.md` — Linux native app
- `docs/superpowers/plans/` — Implementation plans for Windows & Linux native apps
