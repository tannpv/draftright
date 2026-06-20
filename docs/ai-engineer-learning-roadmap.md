# AI Engineer Learning Roadmap — applied in DraftRight

**Owner:** Tan Nguyen
**Created:** 2026-06-17
**Status:** Parked — start AFTER the Go backend refactor (Phase 4c) is finished.
**Goal:** Learn AI engineering by applying theory to a real shipping LLM product
(DraftRight), one rung at a time. Each rung ships a real feature on its own
branch (brainstorm → spec → plan → build) so you learn *and* improve the app.

---

## Why DraftRight is a good sandbox

DraftRight is already an LLM product: select text → pick a tone → get an AI
rewrite. The AI is a pluggable provider system (`ai_providers` table, types
`openai | anthropic | ollama | custom`, one default+active row, dispatched by
`callProvider(provider, system, userText)`). Prod currently routes to Ollama
Cloud (`gpt-oss:20b-cloud`). That means every core AI-engineering concept has a
natural home here without building a new app.

**Recommended order:** 0 → 1 → 3 → 2 → 4 → 5 → 6.
Eval (rung 3) comes early on purpose — everything after it needs measurement,
and "can you tell if a change made the model better" is the skill that separates
AI engineers from people who just call an API.

---

## Rung 0 — Setup & baseline (prerequisite, ~half day)

**Objective:** Be able to run the model, send a prompt, and read a result
locally — and capture a baseline so later rungs have something to beat.

- Run the backend locally against a provider you control (a personal OpenAI or
  Anthropic key, or local Ollama).
- Trace one rewrite end-to-end: client → backend `callProvider` → provider HTTP
  call → response → client. Write down each hop.
- Capture 20 real (input text, tone, output) triples from the rewrite path as a
  `baseline.jsonl`. This is your first dataset.

**Deliverable:** a working local rewrite + `baseline.jsonl` (20 rows).
**You'll understand:** the full request path, where prompts live, what a
provider config actually controls.

---

## Rung 1 — Prompt engineering / in-context learning (~2-3 days)

**Theory:** system vs user role, zero-shot vs few-shot, instruction clarity,
temperature / top_p, output-format control, prompt injection basics.

**Build in DraftRight:** the tone rewrites (`simple, natural, polished, concise,
technical, claude, grammarCheck, translate`) are just prompts. Improve them:
- Rewrite each tone's system prompt with explicit role + constraints.
- Add 1-2 few-shot examples to the weakest tone; compare with/without.
- Expose and experiment with `temperature` per tone (creative vs factual).

**Touch:** backend rewrite prompt construction + `ai_providers.temperature`.
**Deliverable:** improved prompts + a short writeup: which change helped which
tone, and your hypothesis why.
**You'll understand:** how much behavior is "free" via prompting before you ever
touch weights, and why few-shot works (in-context learning).

---

## Rung 3 — Evaluation harness (~1 week) — DO THIS EARLY

**Theory:** offline eval, golden/reference sets, LLM-as-judge, pairwise
comparison, rubric scoring, regression detection, why human eval is the ground
truth and automated eval is a proxy.

**Build in DraftRight:** there is already a `tone_test` notion — turn it into a
real eval harness:
- Curate a golden set: ~50 (input, tone, ideal-ish output or rubric) rows.
- Implement an LLM-as-judge: a second model scores a rewrite 1-5 against a
  per-tone rubric (e.g. "concise: shorter, meaning preserved, no new facts").
- Produce a scorecard: pass-rate per tone, per provider, with examples of the
  worst failures.
- Re-run it against the Rung 1 prompt changes — now you can *prove* improvement
  instead of guessing.

**Deliverable:** a runnable eval that outputs a per-tone/per-provider scorecard +
a saved baseline run.
**You'll understand:** the single most important AI-eng loop — change → measure →
keep or revert. Also judge bias, rubric design, eval/test split discipline.

---

## Rung 2 — LLM integration & inference internals (~3-4 days)

**Theory:** chat-completions API shape, streaming tokens (SSE), context windows,
tokenization, max_tokens, cost & latency tradeoffs, retries/timeouts/backoff,
graceful degradation.

**Build in DraftRight:**
- Add token counting + cost estimate per rewrite (log it).
- Add streaming for one provider so rewrites appear progressively.
- Harden `callProvider`: timeout, retry with backoff, provider-error
  sanitization (some already exists — extend it).

**Touch:** the provider call layer (NestJS now; the Go port's
`rewrite/adapter/*` + `aicall.Completer` after the refactor).
**Deliverable:** streaming on ≥1 provider + per-rewrite token/cost logging.
**You'll understand:** what an inference call actually costs and how latency is
shaped, why streaming changes UX, how to fail safely.

---

## Rung 4 — Self-hosting & quantization (~1 week)

**Theory:** model serving, GGUF quantization levels (Q4/Q5/Q8), KV cache,
throughput vs latency, CPU vs GPU inference, model size vs quality.

**Build in DraftRight:**
- Stand up Ollama on the self-host box (the new Hostinger KVM 4, or any box with
  a GPU you can borrow). Pull a small model (Llama 3.2 3B) and a quantized
  variant.
- Add it as a `type: ollama` provider, set it active, run the Rung 3 eval
  against it vs Ollama Cloud.
- Benchmark: tokens/sec, latency, and quality-score delta across quant levels.

**Reality check:** KVM 4 is **CPU-only** → inference is slow. That's a *feature*
for learning — you'll feel exactly why quantization and GPUs exist. Keep cloud
as the default provider for real users.

**Deliverable:** a self-hosted provider + a benchmark table (quant level →
speed → eval score).
**You'll understand:** the serving side of LLMs, the quality/cost/speed triangle,
and why "just self-host it" is rarely free.

---

## Rung 5 — RAG / embeddings (~1-2 weeks)

**Theory:** embeddings, vector similarity, chunking, top-k retrieval,
retrieval-augmented prompting, when RAG helps vs hurts, eval for retrieval.

**Build in DraftRight:** a "rewrite in *my* style" feature:
- Embed a user's past accepted rewrites; store vectors (pgvector in Postgres, or
  a vector store).
- On a new rewrite, retrieve the k most similar past samples and inject them as
  few-shot style exemplars.
- Measure with the Rung 3 harness: does style-conditioned rewriting score higher
  on a "sounds like the user" rubric?

**Deliverable:** style-personalized rewrite behind a flag + a retrieval eval.
**You'll understand:** the dominant production pattern for grounding LLMs, and
how to evaluate retrieval separately from generation.

---

## Rung 6 — Fine-tuning & dataset curation (~2-3 weeks)

**Theory:** supervised fine-tuning (SFT), dataset cleaning/dedup, train/eval
split, overfitting, when fine-tuning beats prompting/RAG (and when it doesn't),
LoRA/PEFT basics.

**Build in DraftRight:** this aligns with the roadmap's `training-data` +
AI fix-proposal work (Go Phase 4c-4):
- Curate rewrite logs into a clean SFT set for ONE tone (e.g. "concise"): filter,
  dedup, hold out an eval split.
- Fine-tune a small open model (LoRA on Llama 3.2, or a hosted fine-tune API).
- Serve it as a provider; eval vs the base model + best prompt from Rung 1.

**Deliverable:** a fine-tuned "concise" model + an honest eval showing whether it
actually beat good prompting (often it won't at small scale — that's a real
lesson, not a failure).
**You'll understand:** the full data → train → serve → eval loop, and the
engineer's judgment of *when* fine-tuning is worth it.

---

## Working method (for every rung)

1. Branch from `develop`: `feature/ai-learn-<rung>-<YYYYMMDD>`.
2. Brainstorm → spec → plan → build (the normal project workflow).
3. Each rung must end with a number from the Rung 3 harness — no "feels better".
4. Keep a `docs/ai-learning-journal.md`: hypothesis, change, result, surprise.
   The journal is where the learning compounds.

## Companion theory (read alongside, not before)

- Prompting & in-context learning — provider prompt guides (OpenAI/Anthropic).
- Eval — "LLM-as-judge" literature; pairwise/rubric eval.
- Serving/quantization — llama.cpp / Ollama docs, GGUF quant notes.
- RAG — embeddings + vector DB docs (pgvector).
- Fine-tuning — LoRA/PEFT papers + a hosted SFT quickstart.

Learn each concept *because a rung forces you to*, then read deeper. Theory
sticks when it's debugging a result you already have in front of you.
