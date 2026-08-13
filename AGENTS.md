# AGENTS.md — DraftRight

Instructions for OpenAI Codex and any agent that reads `AGENTS.md`. This file is
a **pointer, not a copy** — the canonical project docs live elsewhere in the repo
so there is one source of truth (RULE #1). Read them; do not restate them here.

## Canonical docs — read first
- **`CLAUDE.md`** (repo root) — project overview, architecture, tech stack, ports,
  quick start, git workflow. Tool-neutral; applies to every agent.
- **Subdirectory `CLAUDE.md`** — auto-scoped context per area:
  `backend/`, `admin/`, `website/`, `DraftRight/` (macOS), `DraftRightMobile/`,
  `DraftRightWindows/`, `DraftRightLinux/`. Read the one for the area you touch.
- **`docs/`** — specs, plans, release runbook (on-demand, not auto-loaded).

## RULE #1 (standing first rule — full text in `CLAUDE.md`)
Clean, Reusable, Extendable, **No hardcoding**. Any value that carries meaning is
an enum/const/config with ONE source of truth — never a literal at a call site.
Duplicated logic counts as hardcoding (two copies drift). Worked example: issue
**#22** — a copy-pasted `app_releases` upsert drifted between manual and CI paths,
shipping Windows installers with no integrity check to prod for two months.

## Before merging — every task
1. Clean garbage — delete what the change orphaned (DELETE, never deprecate).
2. Full review over the diff — correctness, RULE #1, security.
Full 19-step checklist lives in the maintainer's global rules; the repo summary is
in `CLAUDE.md`.

## Git workflow
GitFlow: branch from `develop` (`feature/<desc>-<YYYYMMDD>`), `--no-ff` merge.
Never commit directly to `main`/`develop`. Prefixes: `feat:` `fix:` `chore:` `docs:`.

## Active plans (design-approved, not yet built)
- **#173 — per-user AI personalization** →
  `docs/superpowers/plans/2026-08-13-user-context-personalization.md`.
  Central store + server-side prompt injection in `rewrite.service` (NOT model
  fine-tuning; NOT per-platform copies+sync). Privacy-heavy — Phase 2 ties written
  content to identity; read the plan's privacy section before writing any code.
