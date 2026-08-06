---
name: cleanup-garbage
description: Find and DELETE what a change left behind — dead code, unreferenced symbols, unused DB tables/columns, stale config and env vars, unused deps, orphaned files. Use after implementing a feature or fix, before the full review. Trigger — "clean garbage", "cleanup", "dead code", "what's unused", or step 6 of the Development Task Checklist.
---

# Cleanup Garbage (DraftRight)

Step 6 of the Development Task Checklist, run **after implementing** and
**before the full review**.

Adapted for DraftRight's stack (C# / Python / Swift / Dart / TypeScript / Go +
Postgres). A `cleanup-garbage` skill previously existed in `lingua-app` for a
Go+Flutter monorepo; the phases carry over, the tooling does not.

## Core rule

**DELETE, never deprecate.**

No commented-out code. No `// TODO remove`. No dead-but-kept "we might need it".
If it is superseded, it goes. Git history is the archive.

The one exception: code that is unreferenced **on purpose** because its consumer
is a disclosed follow-up. Do not delete it silently — **state it in the report**
with the issue number that will consume it, so the owner decides.

## Scope first

Run against the change under review, not the whole repo:

```bash
git diff --name-only origin/main..HEAD     # or the task's branch point
```

A whole-repo sweep is a separate, explicitly-requested job — say so rather than
expanding scope unasked.

## Phase 1 — Dead code

Per platform, find symbols nothing references:

```bash
# C# (Windows) — build surfaces CS0169/CS0414; for reachability, check by hand:
#   for each new public/internal type, count refs OUTSIDE its own file
grep -rl "<Symbol>" DraftRightWindows/DraftRightWindows | grep -v "bin/\|obj/"

# Python (Linux)
grep -rn "from draftright.<module> import\|import <module>" DraftRightLinux/draftright

# Swift (macOS), Dart (mobile), TypeScript (backend/admin/website)
grep -rn "<Symbol>" DraftRight/ DraftRightMobile/lib/ backend/src/ admin/src/
```

Classify each new symbol as **prod-referenced**, **test-only**, or **unreferenced**.
Test-only and unreferenced are findings — report them with counts, don't just delete
public API someone may be mid-way through wiring up.

Also check: unused imports, unreachable branches, superseded helpers left beside
their replacement.

## Phase 2 — Dead DB schema

```bash
# Does anything read this column/table?
grep -rn "<column_name>" backend/src backend-rewrite-go/internal
```

Candidates: columns added for a reverted feature, tables no entity maps, rows
belonging to a deleted feature.

**Rules — non-negotiable:**
- A drop ships as a **reversible migration** (up **and** down SQL) in `backend/sql/`.
- Take a backup before applying on prod (`pg_dump`).
- **Never hand-drop on prod.** Run on dev, verify, then prod.
- Prod runs `synchronize: off` — an entity/DB mismatch 500s every query. See
  memory `feedback_prod_synchronize_off_migrations`.

## Phase 3 — Dead config / env / secrets

```bash
grep -rn "<ENV_VAR>" backend/src backend-rewrite-go docker-compose.yml deploy/
```

Env vars outliving their code is a recurring pattern — the `lingua-app`
precedent was `STUDY_CHECK_KEY` and `KANJI_PASS_THRESHOLD` lingering in
docker-compose after the code was deleted. Check `docker-compose.yml`,
`deploy/.env*.example`, `env.schema.ts`, and CI workflow `env:` blocks.

Removing a secret from config does **not** rotate it — say so if one is dropped.

## Phase 4 — Orphaned routes, files, deps

- Routes/handlers no client calls (check all 7 clients, not just one).
- Files nothing imports; assets nothing references.
- Unused deps: `package.json`, `requirements.txt`/`setup.py`, `.csproj`
  `PackageReference`, `Package.swift`, `pubspec.yaml`.
- Committed build output or scratch files:
  ```bash
  git ls-files | grep -E "/(bin|obj)/|__pycache__|\.DS_Store|node_modules"
  ```

## Phase 5 — Verify

Deleting is the easy part; proving nothing broke is the job.

```bash
# Windows — headless gate (issue #80). Build alone is NOT enough.
cd DraftRightWindows/DraftRightWindows.PureTests && dotnet test
# On a net10-only host: DOTNET_ROLL_FORWARD=Major dotnet test
# Compile-check the app from macOS:
dotnet build DraftRightWindows/DraftRightWindows/DraftRightWindows.csproj \
  -p:EnableWindowsTargeting=true -p:WindowsAppSDKSelfContained=false

cd backend && npx tsc --noEmit && npm test
cd backend-rewrite-go && go vet ./... && go test ./...   # vet/test, NOT just build
python3 DraftRightLinux/test/test_diff_and_grammar.py
swift build                                              # macOS
```

Then re-grep for every deleted symbol to confirm zero references remain.

## Phase 6 — Commit

GitFlow (see `~/.claude/CLAUDE.md`): branch from develop as
`chore/cleanup-<area>-<YYYYMMDD>`, one commit per logical removal, `--no-ff`
merge. Never commit directly to develop or main.

Commit body states **what was deleted and how it was proven dead** — a reviewer
must be able to disagree without re-deriving the analysis.

## Report format

```
DELETED
  <path:symbol>  — <why dead, how verified>

UNREFERENCED BUT KEPT (owner's call)
  <path:symbol>  — awaiting #NNN; delete or keep?

CLEAN
  DB schema · config/env · deps · build output
```

Never report "clean" for a category you did not actually check.
