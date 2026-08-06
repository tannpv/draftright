# Implementation plans — required closing phases

Every plan in this directory **must end with these two phases**. A plan without
them is not a finished plan, and the work it describes is not done when the
feature works.

## Final phase — 1. Clean garbage

Delete what the change orphaned: dead code and unreferenced symbols, unused DB
tables/columns/rows, stale config and env vars, unused dependencies, leftover
files and committed build output.

**DELETE, never deprecate** — no commented-out code, no `// TODO remove`, no
dead-but-kept code. Git history is the archive.

DB drops ship as a reversible up/down migration with a backup taken first, and
are never hand-applied on prod (prod runs `synchronize: off`; an entity/DB
mismatch 500s every query).

Run `/cleanup-garbage` — see `.claude/skills/cleanup-garbage/SKILL.md`.

## Final phase — 2. Full review

Run `/epiphanydev:full-review` over the diff: correctness, RULE #1 compliance
(clean / reusable / extendable / no hardcoding), and security. Fix what it finds
**before** merging to develop.

## Scope

Applies to every task state — new, in progress, merged-but-unfinished, and
anything still sitting in the backlog. "It's a small change" and "the rest is
already merged" are not exemptions; garbage accumulates precisely in the tasks
people consider already done.

These are steps 6 and 7 of the 19-step Development Task Checklist in the
maintainer's `~/.claude/CLAUDE.md`, which remains canonical. This file exists so
a plan author sees the requirement without leaving the repo — do not restate the
rest of the checklist here.

## Note on existing plans

Plans predating 2026-08-06 do not carry these phases explicitly. They are
records of completed work and were left as written rather than rewritten after
the fact. The requirement applies to any plan authored or resumed from that date
onward.
