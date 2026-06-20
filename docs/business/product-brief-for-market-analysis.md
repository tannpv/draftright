# DraftRight — Product Brief for Market Analysis

> Prepared for an analyst (Claude) to assess market potential. Everything below is factual product/business state as of 2026-06-20. Where a number is configurable rather than fixed, it is flagged "confirm in admin."

---

## 1. One-line description

DraftRight is an AI-powered text-rewriting platform: a user selects any text, picks a *tone* (not a prompt), and gets a polished rewrite in seconds — across desktop, mobile (including a custom keyboard), and web.

**Core thesis:** Remove prompt-engineering from AI writing. Users never write a prompt. They pick from a small set of carefully tuned tones and the system does the rest. Positioned for "anyone who types," explicitly including non-native English speakers.

---

## 2. What the product does

- **Select → pick tone → rewrite.** Output replaces the original in place, or can be copied/undone.
- Works on a text *selection* anywhere the platform allows (system-wide on desktop via hotkey/panel; via a custom keyboard or share-extension on mobile; in a playground on web).
- No account required to try (web playground offers free rewrites with no sign-up).

### Tone / mode catalog (current, 8 modes)
| Mode | Kind | Purpose |
|---|---|---|
| Simple | rewrite | Plain, clear |
| Natural | rewrite | Conversational |
| Polished | rewrite | Professional/refined — emails, proposals, presentations |
| Concise | rewrite | Tighten, remove redundancy |
| Technical | rewrite | Technical register |
| Claude | rewrite | Claude-flavored rewrite |
| Grammar Check | grammar | Grammar correction |
| Translate | translate | Translation |

Tones are server-defined (single source: `GET /rewrite/tones`), so the catalog can expand without client releases.

---

## 3. Platforms (7) — unusually broad surface area

| Platform | Tech | Distribution status |
|---|---|---|
| **macOS** native | Swift / SwiftUI / AppKit, macOS 13+ | Live (App Store) |
| **iOS** | Flutter + Swift custom keyboard & share extensions | In stores / active releases (v2.4.1) |
| **Android** | Flutter + Kotlin custom keyboard extension | Sideloaded + Play closed testing; not yet public-store live |
| **Windows** native | WinUI 3, C# 12, .NET 8, MSIX | Live (Microsoft Store) |
| **Linux** native | GTK4 / libadwaita / Python 3.11+ | Code-complete |
| **Web** | Astro 5 + React islands | Live (marketing + playground) |
| **(MS Word add-in)** | Planned, MVP spec written, not built | Not started |

The **custom mobile keyboard** is a key differentiator: rewrite happens inside any app's text field (iMessage, WhatsApp, email, social) without switching apps. Includes a full multi-language IME (Vietnamese Telex, Japanese romaji→kana, Korean Hangul, Chinese pinyin in progress) — meaning it doubles as a real keyboard, not just a rewrite button.

---

## 4. Technology / AI

- **Backend:** originally NestJS (TypeScript); now a byte-identical Go port in production (cutover complete 2026-06-19). PostgreSQL 16 + Redis.
- **AI providers (pluggable):** OpenAI, Anthropic (Claude), Ollama (local/self-host), or any OpenAI-compatible API. Provider switchable in admin without redeploy.
- **Default provider:** OpenAI in production; Ollama Llama 3.2 supported for free/local/self-hosted operation.
- **Admin portal** (React) for provider config, user/subscription management, payments, error triage, feedback board.

**Cost/margin implication:** because the provider is pluggable down to local Ollama, gross margin per rewrite is controllable — can run on cheap/local models for free-tier users and premium models for paid.

---

## 5. Monetization

- **Free plan:** 10 rewrites/day (was 20, lowered to 10 — confirm in admin), $0.
- **Pro plan:** paid, monthly **and** yearly billing periods. Exact price stored in DB (configurable; confirm current amount in admin — not hardcoded).
- **Freemium model** with daily-limit gating + subscription upgrade. Includes subscription nudges, expiry→downgrade-to-Free automation, in-app cancel.

### Payment infrastructure (already built, multi-rail)
- **Lemon Squeezy** (Merchant of Record) — primary, chosen because a Vietnamese individual can't directly use Stripe. Handles global tax/compliance.
- **Stripe** — incl. **Apple Pay + Google Pay** (both verified end-to-end on real devices).
- **PayPal, Momo, VietQR (MB Bank), Bank Transfer** — configurable.
- Payment strategy is a unified pattern across all 4 native/mobile clients; adding a method = 1 enum + 1 handler.

**Note:** MoR (Lemon Squeezy) means no merchant entity / tax-registration barrier to selling globally — significant for a solo founder in Vietnam.

---

## 6. Target users (as positioned on the site)

"Students, professionals, writers, non-native speakers — anyone who types."

Strongest wedge segments to evaluate:
1. **Non-native English speakers** — the keyboard + tone model removes both the writing-quality gap and the prompt-skill gap. Large global TAM (ESL writers in email/chat/work).
2. **Professionals writing email/proposals** — "Polished" + "Concise" tones.
3. **Mobile-first messaging users** — the in-keyboard rewrite is the differentiator vs. desktop-bound competitors.

---

## 7. Current status

- **Production live:** backend (Go) on DigitalOcean Singapore, HTTPS, behind Caddy; api.draftright.info. Admin + marketing site deployed.
- **App store presence:** macOS + Windows live; iOS active; Android in closed testing.
- **Maturity:** payments verified end-to-end (real purchases), auth (incl. Google + Sign in with Apple), email delivery (DNS/deliverability configured), account deletion, forgot/reset password, feedback board — all shipped. This is a *operational* product, not a prototype.
- **Ownership:** solo side project by the founder (Tan Nguyen). Not yet disclosed to employer; IP risk unverified — **factor into go-to-market/funding timing.**

---

## 8. Competitive landscape (for the analyst to expand)

Direct/adjacent competitors to benchmark:
- **Grammarly / GrammarlyGO** — dominant, grammar + AI rewrite, has mobile keyboard. Main incumbent.
- **Apple Intelligence Writing Tools / Microsoft Copilot / Google "Help me write"** — OS-level, free, bundled. Biggest platform-risk threat.
- **Wordtune, QuillBot, DeepL Write** — rewriting/paraphrase niche.
- **Notion AI, Compose AI** — embedded-in-app rewrite.

**DraftRight differentiation to test:**
1. Breadth — 7 platforms incl. native desktop apps *and* a custom mobile IME, where most rivals are browser-extension or single-OS.
2. Tone-not-prompt UX (no prompt engineering).
3. Provider-agnostic incl. self-host/local (privacy angle — text never leaves device on Ollama).
4. Built-in multilingual keyboard (VI/JA/KO/ZH) — bundles "good keyboard" + "AI rewrite," strong for Asian/non-native markets.

**Key risks to size:**
- OS-bundled free writing tools (Apple/Microsoft/Google) commoditizing the core feature.
- Grammarly's brand + distribution moat.
- Solo-founder bandwidth across 7 platforms.
- Per-rewrite LLM cost vs. free-tier abuse (mitigated by Ollama fallback + daily limits).

---

## 9. Questions for the market analysis to answer

1. Which wedge segment (non-native ESL vs. mobile messaging vs. pro email) has the best CAC/LTV and least incumbent overlap?
2. Is the 7-platform breadth a moat or a focus-diluting liability for a solo founder?
3. How defensible is "tone-not-prompt" once OS vendors ship free equivalents?
4. Pricing power: what will the target segment pay vs. Grammarly's ~$12/mo and free OS tools?
5. Geographic entry: lead with Vietnam/SEA (founder's market, MoR-friendly, large ESL base) or Western markets?
6. Is the local/self-host (Ollama) privacy angle a viable B2B/enterprise wedge?

---

*Numbers flagged "confirm in admin" (free-tier limit, Pro price) should be verified against the live production DB before quoting externally.*
