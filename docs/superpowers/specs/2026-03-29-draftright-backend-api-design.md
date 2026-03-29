# DraftRight Backend API — Design Spec

**Date:** 2026-03-29
**Status:** Approved
**Sub-project:** 1 of 4 (Backend API)

## Overview

NestJS backend API that sits between DraftRight clients (mobile, desktop, macOS) and AI providers (OpenAI, Ollama, custom). Handles user authentication, subscription management, usage tracking with daily limits, and AI rewrite proxying. Includes admin CRUD endpoints for the admin portal.

Deployed as Docker containers (NestJS + PostgreSQL) via Docker Compose.

## Architecture

```
Clients (Mobile/Desktop/macOS)
        │
        ▼
┌──────────────────────────────┐
│   NestJS Backend (port 3000) │
│                              │
│  ┌──────────┐ ┌───────────┐ │
│  │ Auth     │ │ Rewrite   │ │
│  │ Module   │ │ Module    │──── ▶ OpenAI / Ollama / Custom
│  ├──────────┤ ├───────────┤ │
│  │ User     │ │ Usage     │ │
│  │ Module   │ │ Module    │ │
│  ├──────────┤ ├───────────┤ │
│  │ Sub      │ │ AI Prov   │ │
│  │ Module   │ │ Module    │ │
│  ├──────────┤ ├───────────┤ │
│  │ Admin    │ │ Plan      │ │
│  │ Module   │ │ Module    │ │
│  └──────────┘ └───────────┘ │
│         │                    │
│    ┌────▼──────┐             │
│    │PostgreSQL │             │
│    │(port 5432)│             │
│    └───────────┘             │
└──────────────────────────────┘
```

## Tech Stack

- **Runtime:** Node.js 20+
- **Framework:** NestJS 10+
- **Language:** TypeScript
- **Database:** PostgreSQL 16
- **ORM:** TypeORM
- **Auth:** JWT (access + refresh tokens), bcrypt for password hashing
- **Validation:** class-validator, class-transformer
- **API Docs:** Swagger (auto-generated)
- **Containerization:** Docker + Docker Compose

## Database Schema

### users

| Column | Type | Notes |
|---|---|---|
| id | uuid | PK, auto-generated |
| email | varchar(255) | unique, indexed |
| password_hash | varchar(255) | bcrypt |
| name | varchar(255) | |
| is_active | boolean | default true |
| role | enum(user, admin) | default user |
| created_at | timestamp | |
| updated_at | timestamp | |

### plans

| Column | Type | Notes |
|---|---|---|
| id | uuid | PK |
| name | varchar(100) | e.g., "Free", "Monthly Pro", "Yearly Pro" |
| daily_limit | int | -1 = unlimited |
| price_cents | int | 0 for free tier |
| billing_period | enum(none, monthly, yearly) | none = free |
| is_active | boolean | default true |
| created_at | timestamp | |
| updated_at | timestamp | |

Seed data: Free plan (daily_limit=10, price=0, billing_period=none)

### subscriptions

| Column | Type | Notes |
|---|---|---|
| id | uuid | PK |
| user_id | uuid | FK → users |
| plan_id | uuid | FK → plans |
| status | enum(active, cancelled, expired) | |
| store_type | enum(google_play, apple_iap, admin_granted) | |
| store_transaction_id | varchar(500) | nullable, receipt ID |
| started_at | timestamp | |
| expires_at | timestamp | nullable (null = never for free) |
| created_at | timestamp | |
| updated_at | timestamp | |

New users automatically get a subscription to the Free plan (status=active, store_type=admin_granted, expires_at=null).

### ai_providers

| Column | Type | Notes |
|---|---|---|
| id | uuid | PK |
| name | varchar(255) | display name |
| type | enum(openai, ollama, custom) | |
| endpoint_url | varchar(500) | |
| api_key | varchar(500) | encrypted at rest |
| model | varchar(100) | e.g., "gpt-4o-mini" |
| temperature | decimal(3,2) | default 0.3 |
| is_default | boolean | only one can be default |
| is_active | boolean | default true |
| created_at | timestamp | |
| updated_at | timestamp | |

Seed data: one OpenAI provider (endpoint=https://api.openai.com/v1/chat/completions, model=gpt-4o-mini, is_default=true). API key set via env var on first run.

### usage_logs

| Column | Type | Notes |
|---|---|---|
| id | uuid | PK |
| user_id | uuid | FK → users, indexed |
| tone | varchar(20) | e.g., "simple", "natural" |
| input_length | int | character count |
| output_length | int | character count |
| ai_provider_id | uuid | FK → ai_providers |
| response_time_ms | int | |
| created_at | timestamp | indexed, for daily counting |

Daily usage count query: `SELECT COUNT(*) FROM usage_logs WHERE user_id = ? AND created_at >= today_start`

## API Endpoints

### Auth Module

**POST /auth/register**
- Body: `{ email, password, name }`
- Validates email format, password min 8 chars
- Creates user + free plan subscription
- Returns: `{ access_token, refresh_token, user: { id, email, name } }`

**POST /auth/login**
- Body: `{ email, password }`
- Returns: `{ access_token, refresh_token, user: { id, email, name } }`

**POST /auth/refresh**
- Body: `{ refresh_token }`
- Returns: `{ access_token, refresh_token }`

**POST /auth/change-password**
- Auth: JWT required
- Body: `{ current_password, new_password }`
- Returns: `{ success: true }`

Access token expires in 15 minutes. Refresh token expires in 7 days.

### Rewrite Module

**POST /rewrite**
- Auth: JWT required
- Body: `{ text: string, tone: string, target_language?: string }`
- Flow:
  1. Verify user is active
  2. Get user's subscription → plan → daily_limit
  3. Count today's usage. If >= daily_limit and daily_limit != -1, return 429
  4. Get default active AI provider
  5. Proxy request to AI provider with the tone's system prompt
  6. Log usage to usage_logs
  7. Return result
- Response: `{ rewritten_text: string, usage_today: number, daily_limit: number }`
- Error 429: `{ error: "Daily limit reached", usage_today: number, daily_limit: number }`

### Subscription Module

**GET /subscription**
- Auth: JWT required
- Returns: `{ plan: { name, daily_limit, billing_period }, status, expires_at, usage_today }`

**POST /subscription/verify-receipt**
- Auth: JWT required
- Body: `{ store_type: "google_play" | "apple_iap", receipt_data: string, product_id: string }`
- Flow:
  1. Validate receipt with Google Play / Apple server
  2. Map product_id to a plan
  3. Create or update subscription
- Returns: `{ subscription: { plan, status, expires_at } }`

### Admin Module

All admin endpoints require JWT with role=admin. Returns 403 for non-admin users.

**GET /admin/stats**
- Returns: `{ total_users, active_subscriptions, rewrites_today, rewrites_this_month }`

**GET /admin/users?search=&page=&limit=**
- Returns: paginated user list with current plan and today's usage count

**GET /admin/users/:id**
- Returns: user detail + subscription + recent usage

**PATCH /admin/users/:id**
- Body: `{ is_active?, role?, name? }`
- Returns: updated user

**CRUD /admin/plans**
- GET /admin/plans — list all plans
- POST /admin/plans — create plan `{ name, daily_limit, price_cents, billing_period }`
- PATCH /admin/plans/:id — update plan
- DELETE /admin/plans/:id — soft delete (set is_active=false)

**CRUD /admin/ai-providers**
- GET /admin/ai-providers — list all
- POST /admin/ai-providers — create `{ name, type, endpoint_url, api_key, model, temperature }`
- PATCH /admin/ai-providers/:id — update
- DELETE /admin/ai-providers/:id — soft delete
- POST /admin/ai-providers/:id/test — test connection (sends a simple rewrite, returns success/error)

**POST /admin/subscriptions/grant**
- Body: `{ user_id, plan_id, expires_at? }`
- Manually grants a plan to a user (store_type=admin_granted)

## NestJS Module Structure

```
src/
├── main.ts
├── app.module.ts
├── config/
│   └── database.config.ts
├── auth/
│   ├── auth.module.ts
│   ├── auth.controller.ts
│   ├── auth.service.ts
│   ├── jwt.strategy.ts
│   ├── jwt-auth.guard.ts
│   └── dto/
│       ├── register.dto.ts
│       └── login.dto.ts
├── users/
│   ├── users.module.ts
│   ├── users.service.ts
│   └── entities/
│       └── user.entity.ts
├── plans/
│   ├── plans.module.ts
│   ├── plans.service.ts
│   └── entities/
│       └── plan.entity.ts
├── subscriptions/
│   ├── subscriptions.module.ts
│   ├── subscriptions.service.ts
│   ├── subscriptions.controller.ts
│   └── entities/
│       └── subscription.entity.ts
├── rewrite/
│   ├── rewrite.module.ts
│   ├── rewrite.controller.ts
│   ├── rewrite.service.ts
│   └── dto/
│       └── rewrite.dto.ts
├── ai-providers/
│   ├── ai-providers.module.ts
│   ├── ai-providers.service.ts
│   └── entities/
│       └── ai-provider.entity.ts
├── usage/
│   ├── usage.module.ts
│   ├── usage.service.ts
│   └── entities/
│       └── usage-log.entity.ts
├── admin/
│   ├── admin.module.ts
│   ├── admin.controller.ts
│   ├── admin-auth.guard.ts
│   └── dto/
│       ├── grant-subscription.dto.ts
│       └── update-user.dto.ts
└── common/
    ├── decorators/
    │   └── roles.decorator.ts
    └── guards/
        └── roles.guard.ts
```

## System Prompts

Same as existing mobile app (defined in Tone enum):

| Tone | System Prompt |
|---|---|
| simple | "Rewrite the following text using simple, easy-to-understand language. Use short sentences and common words. Preserve the original meaning. Return only the rewritten text, no explanations." |
| natural | "Rewrite the following text to sound more natural and conversational, as if spoken by a real person. Remove awkward phrasing and make it flow smoothly. Preserve the original meaning. Return only the rewritten text, no explanations." |
| polished | "Rewrite the following text to be more polished and professional. Improve grammar, word choice, and sentence structure for a refined, workplace-appropriate tone. Preserve the original meaning. Return only the rewritten text, no explanations." |
| concise | "Rewrite the following text to be as concise as possible. Remove unnecessary words, redundancy, and filler while preserving the key meaning. Return only the rewritten text, no explanations." |
| technical | "Rewrite the following text in a technical specification style. Use precise, unambiguous language suitable for documentation, specs, or technical communication. Preserve the original meaning. Return only the rewritten text, no explanations." |
| translate | "Translate the following text into {targetLanguage}. If the text is already in {targetLanguage}, translate it into English instead. Preserve the original meaning and tone. Return only the translated text, no explanations." |

## Docker Compose

```yaml
services:
  backend:
    build: ./backend
    ports:
      - "3000:3000"
    environment:
      - DATABASE_URL=postgresql://draftright:password@postgres:5432/draftright
      - JWT_SECRET=${JWT_SECRET}
      - OPENAI_API_KEY=${OPENAI_API_KEY}
    depends_on:
      postgres:
        condition: service_healthy

  postgres:
    image: postgres:16-alpine
    environment:
      - POSTGRES_USER=draftright
      - POSTGRES_PASSWORD=password
      - POSTGRES_DB=draftright
    volumes:
      - pgdata:/var/lib/postgresql/data
    healthcheck:
      test: ["CMD-ONLY", "pg_isready", "-U", "draftright"]
      interval: 5s
      timeout: 3s
      retries: 5

volumes:
  pgdata:
```

## Seed Data

On first run (via TypeORM migration or seed script):

1. **Free plan** — name="Free", daily_limit=10, price_cents=0, billing_period=none, is_active=true
2. **Admin user** — email from env var `ADMIN_EMAIL`, password from `ADMIN_PASSWORD`, role=admin
3. **Default AI provider** — type=openai, endpoint=https://api.openai.com/v1/chat/completions, model=gpt-4o-mini, api_key from env var `OPENAI_API_KEY`, is_default=true

## Environment Variables

| Variable | Required | Description |
|---|---|---|
| DATABASE_URL | yes | PostgreSQL connection string |
| JWT_SECRET | yes | Secret for signing JWT tokens |
| JWT_REFRESH_SECRET | yes | Secret for refresh tokens |
| OPENAI_API_KEY | yes | Default OpenAI API key (seeded into ai_providers) |
| ADMIN_EMAIL | yes | First admin user email |
| ADMIN_PASSWORD | yes | First admin user password |
| PORT | no | Server port (default 3000) |

## Error Responses

All errors follow consistent format:
```json
{
  "statusCode": 401,
  "message": "Invalid credentials",
  "error": "Unauthorized"
}
```

Key error codes:
- 400 — validation error (bad input)
- 401 — not authenticated
- 403 — not authorized (not admin)
- 429 — daily limit reached
- 502 — AI provider error (upstream failure)
