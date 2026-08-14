# Per-user context / personalization — "a specialist per user"

**Status:** PLAN (not started). Author: Tan, 2026-08-13.

## Vision
Every rewrite should be tailored to *who the user is*. The AI should know the
user's job, industry, audience, and writing style, and apply it whenever they
rewrite — so the same "Polished" tone lands differently for a lawyer, a
marketer, and an engineer. Not per-user model *training* (not viable at this
scale); a per-user **context** injected into the rewrite prompt.

## Grounding (what already exists — reuse, don't rebuild)
- `POST /rewrite` is authenticated and already passes the user id:
  `rewrite.controller.ts` → `rewriteService.rewrite(req.user.id, text, tone, …)`.
  **The injection point exists** — the userId is in hand where the prompt is built.
- `rewrite.service.ts` builds the system+user messages; `ai-providers/strategies/*`
  send them. Context is injected here, once, for all providers.
- `rewrite_logs` (`rewrite-log.entity.ts`) already logs input/output/tone/quality
  — but has **no user_id** (it's global training data). Phase 2 adds user_id.
- `users` entity exists; the profile is a new table keyed by user id (not new
  columns on `users`, so profile data stays isolated + easy to encrypt/clear).

## RULE #1 — Step 0 (before any code)
1. **Restates?** The tone system prompts are owned by the tone config — the
   user context is *additional* system context, not a re-statement of the tone.
   Don't duplicate the provider-message assembly; extend the one in
   `rewrite.service`.
2. **Reuse?** The userId plumbing, the AI-provider strategy interface, the
   secret-at-rest encryption (`enc:v1:`, #50) for the stored context, the
   `rewrite_logs` pipeline for the learned signal.
3. **Third case?** The context builder must compose cleanly: explicit profile +
   learned summary + few-shot examples are three *sources* merged into one
   context block — designed as a list of context contributors, not three
   hard-coded branches.
4. **Literals carrying meaning?** Max examples, summary length, refresh cadence,
   context-token budget → named consts. The injected template → one source.

## Central store vs. per-platform copies — DECIDED: central
The context lives **once**, server-side, and is injected **server-side** in
`rewrite.service` (where `userId` already is). Clients do NOT hold their own copy
of the context to get personalized rewrites — the rewrite comes back already
tailored. So:
- **Central** (chosen): one `user_contexts` row = one source of truth (RULE #1).
  Every client needs only a thin edit UI (`GET/PUT /me/context`); no per-platform
  store, no sync engine. All 7 platforms are personalized the moment the backend
  ships, before any client UI exists.
- **Per-platform copies + sync** (rejected): 7 drifting copies + a sync/conflict
  protocol = the #22 drift failure by design, for zero gain — the rewrite already
  requires the backend, so offline personalization isn't possible regardless (no
  local model). More work than central, not less.
- Clients may keep a **read-through cache** of the profile for offline *display*
  only — a cache, not a source; the server row always wins. Same shape as the
  existing per-user `rewrite-cache.service.ts` (Redis keyed on userId).

## Data model
New table `user_contexts` (one row per user):
| Field | Purpose |
|---|---|
| `user_id` (PK/FK) | owner |
| `job_title`, `industry`, `audience` | explicit profile (from onboarding/Settings) |
| `style_notes` | free-text the user writes ("formal, no emojis, British spelling") |
| `enabled` | opt-in toggle (default off until the user fills it, or a first-run prompt) |
| `learned_summary` | LLM-distilled style summary from history (phase 2), nullable |
| `learned_summary_at` | when it was last distilled |
| `updated_at` | |

Sensitive free-text (`style_notes`, `learned_summary`) is encrypted at rest
(reuse `enc:v1:` #50). The user can **view, edit, and clear** all of it.

## How the context is built (two sources, merged)
1. **Explicit** — the user tells us. A short onboarding step + an editable
   "Personalization" section in Settings (all clients) / the web account page.
   High-signal, immediate, no ML.
2. **Learned** (phase 2) — add `user_id` to `rewrite_logs`; capture **accepted**
   rewrites (the client already knows when the user clicks Replace — send an
   `accepted` signal). Periodically (cadence const) an LLM distills the user's
   accepted history into a short `learned_summary` ("writes concisely, finance
   domain, prefers active voice"). Merged with the explicit profile.
3. **Few-shot** (phase 3) — inject 2–3 of the user's best accepted rewrites as
   examples.

## Prompt injection
In `rewrite.service.rewrite(userId, …)`, before calling the provider:
- load the user's context (skip if `enabled=false` or empty — no-op, zero cost);
- build one **context block** (bounded to a token budget const) and prepend it to
  the system message:
  > "About the person you're writing for: [job] in [industry], writing for
  > [audience]. Style: [style_notes + learned_summary]. Apply this to the
  > rewrite; do not mention it."
- append the few-shot examples (phase 3) as prior turns.

One code path, all providers (it's in the shared service, not per-strategy).

## Privacy & personal data (non-negotiable — the hardest part)

### The core escalation to flag loudly
Today `rewrite_logs` stores input/output with **NO user_id** → content is
de-identified. **Phase 2 ties logged content to an identity.** That converts
anonymous logs into a personal-data store of what people actually wrote (private
messages, possibly health/legal/financial text). This is the single biggest
privacy jump in the feature and must be a deliberate, separately-consented step
— never a silent schema add. Do not add `user_id` to `rewrite_logs` without the
consent + retention + encryption controls below already in place.

### Legal frame
Treat as GDPR/CCPA in scope. Writing content can carry **special-category data
(GDPR Art. 9)** — health, politics, religion — so profiling on it is high-risk.
Lawful basis = **explicit consent**, granular and revocable.

### Consent — two separate, default-OFF toggles
- "Store my profile" (explicit job/industry/style) ≠ "Learn from my rewrites"
  (derive from history). Distinct consents; each default OFF; each revocable.
- Revoking "learn" stops distillation AND purges the retained raw content.

### Data minimization + retention
- Keep the distilled `learned_summary`, **not** raw history. Auto-expire raw
  rewrite content after `RawContentRetentionDays` (const) once distilled.
- User can exclude any single rewrite from learning ("don't learn from this").

### Encryption & access
- Content + `style_notes` + `learned_summary` **encrypted at rest** (#50 `enc:v1:`).
- **Never** returned by admin APIs or logs (ties to #49 admin-readable secrets).

### Third-party AI providers
- Injecting the profile sends extra personal data (job, style, summary) to
  OpenAI/Anthropic on top of the input text they already receive. Requires a DPA
  + provider **no-train** setting enabled.
- **Local Ollama = zero third-party exposure** → this is the privacy tier to
  offer users who don't want content leaving the server.

### User rights (GDPR 15/17)
- View / edit / **export** / clear from Settings. Clear = hard-delete the
  `user_contexts` row **and** purge that user's retained logged content — not a
  flag flip.

### Cross-user isolation
- Learned data derives only from the user's *own* rewrites. Never cross-user.

### Policy
- Update privacy policy + ToS (website `/privacy`) to cover profile + learned
  data + third-party processing **before** any content is tied to identity.

### Open tension — Phase 3 few-shot vs. minimization
Few-shot needs raw accepted rewrites retained, which fights "summary-only +
expire raw". Decision: retain only a **small capped set of user-curated examples**
the user explicitly flags ("save as example") — opt-in, bounded, so retention is
a choice, not a default. Alternative: drop few-shot, summary-only. Prefer curated.

## Phases (each ships value)
| Phase | Scope | Ships |
|---|---|---|
| **1** | `user_contexts` table + explicit profile UI (onboarding + Settings, web first) + inject explicit context in `rewrite.service` | Immediate personalization from what the user tells us |
| **2** | `user_id` + `accepted` on `rewrite_logs`; per-user `learned_summary` distiller (scheduled job); merge into the injected context | Personalization that improves with use |
| **3** | Few-shot: inject the user's best accepted rewrites as examples | Sharpest tailoring |

## Test cases (add to docs/test-cases.xlsx BEFORE coding — checklist step 1)
- Context OFF / empty → rewrite identical to today (no regression, zero cost).
- Explicit profile set → system prompt contains the context block; output reflects
  it (golden-vector: same input+tone, with vs without a "lawyer" profile).
- Context never leaks into the output text ("do not mention it").
- Clearing the profile removes the row + stops injection.
- Encryption: `style_notes` stored `enc:v1:…`, never plaintext in DB/logs.
- Learned summary (phase 2) derives only from the same user's accepted rewrites.
- Token budget respected (long profiles truncated, not unbounded).

## Open questions (decide before build)
- Onboarding: ask upfront, or progressive ("we noticed you rewrite a lot — tell
  us your role for better results")?
- Default `enabled`: off until filled, or a first-run prompt?
- Which client first for the UI — web account page (fastest) or macOS/Windows Settings?
- Phase-2 distiller cadence + cost (per N accepted rewrites vs nightly).

## Not doing
- Per-user model fine-tuning / per-user hosted models (infeasible at this scale).
- Using another user's data for anyone (privacy).
