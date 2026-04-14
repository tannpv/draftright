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
| Admin | `src/admin/` | Admin CRUD for all resources + analytics |

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
