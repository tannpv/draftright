# DraftRight Changelog

## 2026-08-25

### Rule #1 debt cleanup → PRODUCTION (#204, #205, #148)
- Two codebase audits + fixes, all no-behaviour-change with machine guards against regression. **Backend (#204):** `ErrorCode` constants + a `statusByCode` map + a guard test, then a 175-call-site sweep so a mistyped error code is a compile error, not a silent HTTP 500; payment status/method/store-type + `ProviderType` + strategy-method enums adopted at every call site; tone/currency constants. Three "can't-merge" agreement tests (provider names, strategy methods, error codes) now fail CI on drift.
- **Mobile (#205):** a `PrefsKeys` registry for the app↔native-keyboard contract (18 keys + the App-Group channel name — previously retyped at dozens of sites, a silent-drift #22 risk); fixed a real bug where dev builds posted bug-reports/feedback to the **prod** DB; deduped `_detectSource`/URL-normalize/error-parse/timeout; routed the feedback + bug-report (multipart) requests through the shared `ApiClient` chokepoint.
- **Web + admin:** payment method + payment/subscription status literals → per-app single-source consts.
- **Coverage:** `BackendClient`/`ApiClient`/helpers went 0 → 20 mobile unit tests (rewrite, subscription, payment, multipart, 401-refresh).
- Merged develop→main (40 commits) and **deployed the backend refactors to Contabo prod** — verified `/health` 200, `/rewrite/trial` 200, and a bad-tone request returning `400 code:"invalid-input"` (proves the error-code sweep preserved the exact envelope). Rollback anchor `draftright-backend-go:pre-rule1-20260825`.

## 2026-08-24

### NestJS retired — Go is the only backend (#202)
- Prod had run the Go backend since the 2026-06-19 cutover; NestJS remained only as repo dead-weight. Removed `backend/` (169 TS files), the Node CI workflows, the Node-vs-Go shadow-gate tooling, and the compose `backend:` service (renamed the Go service `backend-go`, serving all routes). Branch protection switched to require the four Go CI checks (new `backend-go-ci.yml`); docs reframed to Go-only. Reversibility tag `pre-nestjs-removal`.

## 2026-08-23

### Prod migrated to Contabo + go-live fixes (#192, #198, #199, #200, #201)
- Production moved off the DigitalOcean droplet to a Contabo box (`deploy@169.58.214.18`, compose project `draftright` + edge Caddy). Fresh DB (`app_settings` seeded with a production baseline; `app_releases` intentionally left for the real release pipeline).
- **#192 (security):** the streaming `/v1/rewrite` used a short built-in prompt with no anti-injection guard and broken translate — converged it onto the parity prompt registry; shipped after a 151/151 shadow-gate run, then re-verified on Contabo.
- **#198:** bug-report screenshot uploads 500'd (nonroot container vs root-owned bind-mount) — durable one-shot `bug-reports-perms` init service in the prod compose.
- **#201:** Go rejected `input_kind: null` where Node's `@IsOptional` accepts it — one `optionalField()` helper makes explicit null == absent across all optional properties.
- **#200:** the release-publish script + Go prod runbook still targeted the dead DO host — retargeted to Contabo via env-overridable consts; removed the dead `versions.json` manifest mechanism (the Astro site reads `/updates/latest`).

## 2026-07-02

### Go backend: streaming /v1/rewrite training-data capture → PRODUCTION (#58)
- Streaming `/v1/rewrite` (SSE, Go-only path used by macOS in Go-backend mode) now writes a `rewrite_logs` row on every clean stream finish, carrying the real served model + provider of the winning failover-chain leaf (new `internal/rewrite/provenance` package).
- Deployed dev (api.dev) then prod; verified end-to-end on both (dev rows 126→128, prod 6346→6347 with `model=gpt-4o-mini`). Closes the training-data gap left by #36.

### Prod fix: /v1/rewrite OpenAI 401 (wrong key in env)
- `/opt/draftright/.env` `OPENAI_API_KEY` held the Ollama Cloud key, so every prod streaming rewrite silently failed with OpenAI 401 since the cutover. Replaced with the correct OpenAI key (the `ai_providers` row `OPENAI_PROVIDER_ID` pins). Lesson recorded in `docs/infrastructure.md`: the streaming path reads the key from env, not from the DB row.

### Security: #49 rotation inventory
- Audited prod + dev for secrets that were admin-readable before the #29/#30 masking fix (2026-06-20). Must rotate: OpenAI key (identical dev+prod), Ollama Cloud key, SePay key. LS / Stripe / Resend need no rotation (configured post-fix or env-only). Checklist on issue #49.

## 2026-06-21/22

### Infra: prod droplet downsized 4GB→2GB (#54)
- Migrated to $12 2GB/1vCPU/50GB Singapore droplet via manual clean rebuild; cutover by reassigning the DO Reserved IP `129.212.208.248` — zero DNS changes. Old droplet destroyed. Full runbook + gotchas in `docs/infrastructure.md` and project memory.
- Lemon Squeezy live keys configured in prod `app_settings` (2026-06-23) — subscribe flow fully live.


## 2026-05-13

### Feedback public board (Spec C)
- New page `draftright.info/feedback` — card list of feature requests sorted by votes, status + target-platform filters, "Load more" pagination, inline "+ Suggest a feature" form. Server-fetches the initial page for SEO; React island (`FeedbackBoard`) handles re-fetch, optimistic upvotes (JWT required, `dr_access_token`), and submit. Logged-out visitors see read-only board + "Sign in to vote" tooltip on the upvote buttons.
- Nav link added (`Feedback`); all client "See all requests →" deep-links (Spec B) now land on a real page.

### Feedback / feature-request client surfaces (Spec B)
- "Suggest a feature" form (title + target-platform dropdown + description) added to every client: web playground (`SuggestFeatureWidget`), macOS (menu-bar + Advanced settings), Windows (Settings → Feedback), Flutter iOS/Android (Settings → Help), Linux (Settings + tray). All POST JSON `{kind:"feature", title, target_platform, description, source}` to `/feedback`, attaching the user's Bearer token when signed in.
- Each surface carries a "See all requests →" link to `https://draftright.info/feedback` (board page = Spec C, pending).
- Per-client `FeedbackService` (Swift/C#/Python/Dart) + matching dialog/widget; Flutter ships with 3 unit tests for the payload shape.

## 2026-05-12

### Feedback / feature-request backend (Spec A)
- `bug_reports` gains `kind`/`title`/`target_platform`/`vote_count`/`is_public`; new `feature_votes` table (one vote per user per feature, `vote_count` derived).
- Public `POST /feedback` (bug or feature; JWT optional → user_id), `GET /feedback` (board feed: kind=feature & is_public, votes desc, `?status=`/`?target_platform=` filters), `POST /feedback/:id/vote` (toggle upvote, JWT required).
- Admin bug-reports list/patch gain `kind`+`target_platform` filters and `title`/`target_platform`/`is_public` patch fields; AI fix-proposal cron scoped to `kind='bug'`.
- `POST /bug-reports` (multipart, screenshots) contract unchanged. Migration: `backend/sql/2026-05-12-feedback.sql`. Specs B (native submit forms) + C (public board page) pending.

### Desktop auto-update overhaul (macOS + Windows → 2.2.4)
- Persistent "Update X.Y.Z available" affordance: Windows tray menu item + Settings→Advanced→Updates link; macOS menu-bar item + Settings→Updates button. No longer have to click "Check for Updates" to learn about a release.
- Silent background pre-download: when a check finds an update, the installer/DMG downloads quietly in the background (3 retries, cache-busting, per-attempt timeout — a stalled socket can't hang the progress window anymore). Once staged, the affordance becomes "ready — restart & install" and the install is instant. Install + restart still requires one user click; nothing auto-restarts.
- Both clients prefer the `platforms.<platform>.{version,url}` map from `/updates/latest` over the legacy top-level fields (macOS fully; Windows TODO). Fixes "Windows-only release invisible to Windows clients" and "macOS downloads a stale dmg".
- Backend: `/updates/latest` top-level `version` is now the highest version across all platforms (was hardcoded to macOS's) — defense-in-depth for older clients.
- Known gotcha documented: missing files under `draftright.info/downloads/` return HTTP 200 + a ~28 KB HTML page (not 404) — always verify `content-length`/`content-type` after `release-publish.sh`.
- Added an iOS bug-report integration test (`DraftRightMobile/integration_test/bug_report_test.dart`), runs on the iOS simulator.

## 2026-03-29

### V2 Backend + Admin Portal
- Built NestJS backend API with PostgreSQL (auth, rewrite proxy, subscriptions, usage limits)
- Built React admin portal with Modernize dark theme (dashboard, users, plans, providers, analytics, transactions)
- Updated mobile app + keyboard extensions to use backend instead of direct OpenAI
- Updated macOS app to use backend
- Added Windows/Linux desktop support (Flutter Desktop with system tray + hotkey)
- Renamed to "DraftRight V2" with new bundle IDs for side-by-side install

### V1 Android Keyboard Fix
- Added full QWERTY keyboard to Android IME (was toolbar-only)
- Fixed blank key labels (emoji icons for toolbar + special keys)
- Fixed API key sync between Flutter app and keyboard extension
- Fixed temperature type casting crash

## 2026-03-28

### V1 Initial Release (tag: v1.0)
- macOS menu bar app (Swift/SwiftUI) with NSServices, floating diff panel
- Flutter mobile app with onboarding, settings, test playground
- iOS keyboard extension (Swift)
- Android keyboard extension (Kotlin)
- 6 tones: Simple, Natural, Polished, Concise, Technical, Translate
- Direct OpenAI API integration (user provides own key)
