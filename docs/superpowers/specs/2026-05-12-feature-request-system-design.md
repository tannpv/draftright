# Feature Request System — Design

**Date:** 2026-05-12
**Status:** Approved (brainstorming) — pending per-spec implementation plans
**Owner:** Tan Nguyen

## Goal

Let users propose new features from any DraftRight client (web playground, Windows app, macOS app, Linux app, iOS, Android), browse and upvote each other's requests on a public board, and let the admin triage them. When submitting, the user picks which platform the request is *for* from a dropdown (Playground / Mobile / Windows / macOS / Linux). Built by **extending** the existing bug-reports pipeline rather than building a parallel system.

## Decisions (from brainstorming)

| # | Question | Decision |
|---|---|---|
| Q1 | Relation to bug-reports | **Extend** — same module/table with a `kind` discriminator |
| Q2 | Visibility | **Public board + upvotes** |
| Q3 | Where the board lives / submit surfaces | **Website-hosted board** (`draftright.info/feedback`) + **native submit forms** in every client (web, Windows, macOS, Linux, iOS, Android); native apps deep-link out to browse |
| Q4 | Who can submit / vote | **Login required** for both; logged-out visitors see the board read-only |
| Q5 | Target platform of the request | User picks from a **dropdown** in the submit form: `playground \| mobile \| windows \| mac \| linux`. Distinct from `source` (auto-detected: where they submitted *from*). |

## Core principles applied

- **Reusable** — no new parallel system. The `feedback` module *is* the bug-reports module extended with a `kind` column. One table, one admin inbox, one DTO/service/controller layer. Native submit sheets are one shared widget per platform parametrized by `kind`, not a copy of the bug sheet.
- **Extendable** — schema carries `status`, `is_public`, `vote_count` from day one, so a roadmap view, AI-clustering of duplicates, or feature categories are additive (no migration churn). `POST /bug-reports` is kept as an alias so existing mobile builds (2.2.2+38) don't break when the module is renamed.
- **Clean code** — separate `CreateFeedbackDto` / `VoteDto`; vote endpoint idempotent with a single source of truth (`vote_count = COUNT(feature_votes)`, never hand-incremented); public vs admin routes split by guard; the AI-fix cron stays scoped to `kind=bug` (no tangled conditionals).

## Architecture

### Backend (NestJS) — `feedback` module

Reuse the `bug_reports` table (TypeORM `@Entity('bug_reports')` — **no table rename**, migration only adds columns):

| Column | Type | Notes |
|---|---|---|
| `kind` | enum `'bug' \| 'feature'` | default `'bug'` (back-compat) |
| `title` | varchar(80) nullable | feature only |
| `target_platform` | enum `playground \| mobile \| windows \| mac \| linux` nullable | feature only; user-picked in the submit dropdown. Distinct from `source` (where submitted from). |
| `status` | enum `open \| planned \| in_progress \| done \| declined` | default `open` |
| `vote_count` | int | default `0`, derived (never hand-set) |
| `is_public` | bool | default `true` — admin's spam/dedupe lever |

New table `feature_votes`:

| Column | Type |
|---|---|
| `id` | uuid PK |
| `feature_id` | FK → `bug_reports.id`, ON DELETE CASCADE |
| `user_id` | FK → `users.id`, ON DELETE CASCADE |
| `created_at` | timestamptz |
| | **UNIQUE(`feature_id`, `user_id`)** |

Module rename → `feedback.module.ts` / `FeedbackController` / `FeedbackService` (clean-code). Keep a thin `POST /bug-reports` alias controller route delegating to the same service so old clients keep working.

Routes:

| Method | Path | Auth | Behavior |
|---|---|---|---|
| `POST` | `/feedback` | JWT optional | body `{kind, title?, target_platform?, description, source, user_email?}`; if JWT present, sets `user_id`. IP rate-limited. Validates: `title` 3–80 chars + `target_platform` ∈ enum (both required when `kind=feature`), `description` ≤2000. |
| `GET` | `/feedback` | none | `kind=feature AND is_public=true`, sorted `vote_count DESC, created_at DESC`, `?status=` filter, paginated (`?page=&limit=`), 60s response cache. Includes `viewerHasVoted` only when a JWT is supplied. |
| `POST` | `/feedback/:id/vote` | **JWT required** | toggles the caller's row in `feature_votes`, then recomputes `vote_count = COUNT(*)`. Idempotent; returns `{vote_count, hasVoted}`. |
| `GET` | `/admin/feedback` | admin guard | all rows, `?kind=`/`?status=`/`?target_platform=` filters, paginated. |
| `PATCH` | `/admin/feedback/:id` | admin guard | `status`, `is_public`, `title`, `target_platform`. |
| `POST` | `/bug-reports` | (existing) | **alias** → `FeedbackService.create({...body, kind:'bug'})`. Unchanged contract. |

The AI-fix cron keeps its existing query (`kind = 'bug'` filter added) — feature requests are never auto-processed.

### Clients — native submit sheets (mirror existing bug-report sheet)

Each client gets a "Suggest a feature" entry and a "See all requests →" link that opens `https://draftright.info/feedback` in the system browser. The submit form has three fields: **title**, **target-platform dropdown** (Playground / Mobile / Windows / macOS / Linux — pre-selected to the platform the user is currently on, but changeable), and **description**.

| Client | Submit surface | `source` value | Browse link |
|---|---|---|---|
| macOS | `FeatureRequestSheet` SwiftUI, reachable from menu-bar item + Settings | `macos-app` | `NSWorkspace.shared.open(url)` |
| Windows | WinForms dialog mirroring the bug-report dialog; tray item + Settings entry | `windows-app` | `Process.Start(url)` |
| Linux | GTK4 dialog (libadwaita), mirroring the bug-report dialog; menu + Settings entry | `linux-app` | `Gio.AppInfo.launch_default_for_uri(url)` |
| iOS + Android | `showFeatureRequestSheet` mirroring `report_bug_sheet.dart`; Settings entry | `ios-app` / `android-app` | `launchUrl(url)` |
| Web playground | inline modal on the playground page; link to `/feedback` | `web` | in-page link |

All post to `POST /feedback` with `kind:'feature'`, the selected `target_platform`, and the user's JWT when logged in. The dropdown's default = the current client's natural platform (`mac` on macOS, `mobile` on iOS/Android, `playground` on web, etc.); the website's on-page form defaults to `playground`.

### Website — public board `draftright.info/feedback`

Astro page, server-fetches `GET /feedback`. Renders a card list: `title`, description excerpt, vote count + upvote button, `status` badge, **`target_platform` badge**. Filter tabs: status (All / Open / Planned / In progress / Done) **and** a platform filter (All / Playground / Mobile / Windows / macOS / Linux). A React island handles the upvote action — needs a JWT, reuses the playground's `localStorage` token; logged-out users see "log in to vote". A submit form on the same page (title + target-platform dropdown + description) posts with `source:web`.

### Admin portal (React)

Extend the existing bug-reports page; nav label becomes "Feedback". Add `kind` and `target_platform` columns + filters (kind: All / Bug / Feature; platform: All / Playground / Mobile / Windows / macOS / Linux). For feature rows: editable `status` dropdown (→ `PATCH /admin/feedback/:id`), `is_public` toggle, `vote_count`, `title`, `target_platform`. Toggling `is_public` off is the dedupe/spam tool.

## Data flow

1. User taps "Suggest a feature" in any client → native sheet / web form (title + target-platform dropdown + description) → `POST /feedback {kind:'feature', title, target_platform, description, source, user_email?}` (JWT attached if logged in → backend sets `user_id`).
2. Backend inserts a `bug_reports` row (`kind=feature, status=open, vote_count=0, is_public=true`).
3. `draftright.info/feedback` lists public feature rows sorted by votes; users filter by status and/or target platform.
4. Logged-in visitor clicks upvote → `POST /feedback/:id/vote` → insert/delete a `feature_votes` row → `vote_count = COUNT(*)` recomputed.
5. Admin triages in `/admin/feedback`: sets `status` (planned / in_progress / done / declined), toggles `is_public` to hide spam/dupes, optionally edits `title` / `target_platform`.

## Error handling

- Validation as above; `400` on violations.
- `POST /feedback` IP rate-limited → `429` on flood.
- Vote endpoint idempotent — re-voting toggles off; concurrent votes safe via the UNIQUE constraint + recompute.
- Public `GET /feedback` cached 60s (stale board acceptable).
- `PATCH /admin/feedback/:id` returns `404` if the row doesn't exist, `400` on invalid `status`.

## Testing

- Backend e2e: create `kind=bug` and `kind=feature` (feature requires `title`+`target_platform`, `400` without); vote toggle + double-vote dedupe; public-list filtering (`is_public=false` hidden, status filter, target_platform filter, sort order); admin `PATCH` (status, is_public, title, target_platform); `POST /bug-reports` alias still works.
- Flutter integration test mirroring `bug_report_test.dart` for the feature-request sheet (asserts the `target_platform` field is sent).
- Manual smoke: macOS sheet → row in `/admin/feedback`; Windows sheet; Linux sheet; web playground modal; website board renders + platform/status filters + upvote works when logged in, read-only when not.

## Decomposition — three specs

This spans backend + 5 clients + website + admin — too large for one implementation plan. Split, built in order:

- **Spec A — backend `feedback` module + schema + admin UI.** Migration (incl. `target_platform` enum), `feedback` module rename + new routes + `bug_reports` alias, `feature_votes` table, admin page changes (kind + target_platform columns/filters), backend e2e tests. *Foundation — B and C depend on it.*
- **Spec B — native submit surfaces.** macOS `FeatureRequestSheet`, Windows dialog, Linux GTK4 dialog, Flutter `showFeatureRequestSheet`, web-playground modal — each with the title + target-platform dropdown + description fields — plus the "See all requests →" deep-links. Parallelizable per platform once A is live. (Flutter integration test lands here.)
- **Spec C — public `/feedback` board.** Astro page, card list with platform badges, status + platform filter tabs, React upvote island, on-page submit form.

Each spec gets its own implementation plan. Start with Spec A.
