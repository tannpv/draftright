# DraftRight Backend

NestJS API server — auth, rewrite proxy, subscriptions, usage tracking, admin CRUD.

## Modules

| Module | Files | Purpose |
|---|---|---|
| Auth | `src/auth/` | JWT register/login/refresh, bcrypt passwords |
| Users | `src/users/` | User entity, CRUD, search/pagination |
| Plans | `src/plans/` | Subscription plan definitions |
| Subscriptions | `src/subscriptions/` | Plan assignment, receipt verification |
| AI Providers | `src/ai-providers/` | OpenAI/Ollama proxy, provider management |
| Rewrite | `src/rewrite/` | Core proxy — quota check → AI call → usage log |
| Usage | `src/usage/` | Daily usage counting, logging |
| User Context | `src/user-context/` | Per-user personalization (#173): `GET/PUT/DELETE /me/context`; `style_notes` encrypted; `buildContextPreamble` injected in Rewrite. **Mirrored in Go** (`internal/usercontext/`). |
| Admin | `src/admin/` | Admin CRUD for all resources + analytics |
| Feedback | `src/bug-reports/` | `POST /feedback` (bug or feature request; JWT optional → user_id), `GET /feedback` (public board: kind=feature & is_public, votes desc, `?status=`/`?target_platform=` filters), `POST /feedback/:id/vote` (toggle upvote, JWT required). `feature_votes` table = one vote per user per feature; `bug_reports.vote_count` is derived. Legacy multipart `POST /bug-reports` (screenshots) unchanged. AI fix-proposal cron only touches `kind='bug'`. Schema migration: `backend/sql/2026-05-12-feedback.sql`. |

## ⚠️ Prod is Go-only — new routes MUST be added to `backend-rewrite-go`

Prod + dev route **all** api traffic to the Go backend (`api.draftright.info`→:3001,
`api.dev`→:3101); this NestJS server is retired in prod (no prod Node container).
So a **new endpoint added only here is unreachable in prod** — it 404s. Every new
route must also be implemented in Go (handler + sqlc query + `shared.Router` field +
`cmd/server/main.go` wiring). This NestJS impl is kept as **rollback parity** only.
(#173 `/me/context` was built Node-only first and 404'd until the Go CRUD landed.)

## Database

- PostgreSQL 16, TypeORM with `synchronize: true` (dev) / migrations (prod)
- Tables: `users`, `plans`, `subscriptions`, `ai_providers`, `usage_logs`
- UUIDs for all primary keys
- Timestamps: `created_at`, `updated_at`

## Auth

- JWT access token: 15min expiry
- JWT refresh token: 7 days expiry
- Passwords: bcrypt with 10 rounds
- Admin role: `role` enum on users table, guarded by `RolesGuard`

### Extension tokens (`dr_ext_*`)

Long-lived (sliding 90-day) per-device tokens for the iOS keyboard,
iOS share extension, and Android keyboard. Backend stores
`sha256(token)` only — never plaintext.

| Endpoint | Auth | Notes |
|---|---|---|
| `POST   /auth/extension-tokens` | Session JWT | Mints / rotates a token for a `(user, device_id)` pair. Returns `{ token, id }`. Plaintext is exposed here only. |
| `GET    /auth/extension-tokens` | Session JWT | Lists active tokens for current user. Server-only fields (`token_hash`, `user_id`) are stripped from the response. |
| `DELETE /auth/extension-tokens/:id` | Session JWT | Revokes a single token; service-level filter on `user_id` prevents cross-user revocation. |
| `POST   /rewrite` | `RewriteAuthGuard` | Accepts a regular session JWT **or** a `dr_ext_*` token with `rewrite` scope. All other protected endpoints keep using `JwtAuthGuard`, so an extension token can never reach `/auth/me`, `/payment`, `/admin`, etc. |

Table: `extension_tokens` (id, user_id FK, token_hash CHAR(64), scopes
TEXT[], device_id, device_name, last_used_at, created_at, revoked_at)
with partial unique indexes on `token_hash` and `(user_id, device_id)`
WHERE `revoked_at IS NULL`.

## Key Patterns

- Entities in `module/entities/entity.ts`
- DTOs with class-validator decorators in `module/dto/`
- Services injected via constructor, exported from modules
- Controllers use `@UseGuards(JwtAuthGuard)` for protected routes
- Admin endpoints use `@UseGuards(JwtAuthGuard, RolesGuard)` + `@Roles('admin')`

## Commands

```bash
npm run start:dev     # Dev server with hot reload
npm run build         # Production build
npm run seed          # Seed free plan + admin + default provider
npx ts-node src/seed.ts  # Alternative seed command
```

## Environment Variables

`DATABASE_URL`, `JWT_SECRET`, `JWT_REFRESH_SECRET`, `OPENAI_API_KEY`, `ADMIN_EMAIL`, `ADMIN_PASSWORD`, `PORT`

## Tone System Prompts

Defined in `src/rewrite/rewrite.service.ts` — 7 tones: simple, natural, polished, concise, technical, claude, translate.

## Secret encryption at rest (#50)

`ai_providers.api_key` + 12 `app_settings` secret columns are encrypted with
AES-256-GCM (`enc:v1:` prefix) via `src/common/crypto/secret-cipher.ts` (Node)
and `backend-rewrite-go/internal/platform/secretcipher` (Go, same wire format —
they share the DB). **Key-gated:** unset `SECRETS_ENCRYPTION_KEY` → no-op /
plaintext. Node uses a TypeORM ValueTransformer on the app_settings columns
(widened to `text` — ciphertext overflows varchar); Go decrypts at use
(completer, payment `Credentials()`, email) and encrypts at write (repos).
Code complete on develop; NOT enabled. Enable runbook + gotchas: memory
`feedback_secret_encryption_50`. Adding a new secret column = encrypt at its
Go write + decrypt at every Go read + Node transformer + widen migration +
add it to `SETTINGS_ENCRYPTED_COLUMNS` (admin/mask-settings.util.ts — the single
source of truth the backfill script imports; `mask-settings.util.spec` fails if
it drifts from the entity's transformer columns).

## AI Providers

`AiProviderType`: openai, anthropic, ollama, custom, **google**. All except anthropic share `OpenAiStrategy` (one OpenAI-compatible wire) — Gemini (`google`) uses its `/v1beta/openai/chat/completions` endpoint. `type` is a **pg native enum** → new values need a `sql/…ALTER TYPE ai_providers_type_enum ADD VALUE…` migration (synchronize OFF in prod). Strategy dispatch: `ProviderStrategyRegistry.pick()`. **The Go backend (`backend-rewrite-go`) has a parallel `completer_factory.go` switch — add new provider types THERE too**, or prod `/rewrite` (Go) errors. Rewrite quality is a function of the provider's `model`; prod Go pins one provider via `OPENAI_PROVIDER_ID` env, so the model that matters is that pinned row's, not the admin "default".
