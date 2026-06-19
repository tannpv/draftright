# Phase 5 Cutover Runbook — Go backend shadow gate → Caddy flip

Goal: prove the Go backend byte-identical to the NestJS authority across all
101 canonical routes via the shadow gate, then flip prod by swapping the whole
Caddy upstream (Node port → Go port). Ship nothing until the live gate is green.

## Fixture coverage — STATIC GREEN (this dev machine)

`cmd/shadowdiff` loads **129 fixtures** across 9 module dirs and covers all
**101 canonical routes** in `deploy/shadow/routes.txt` with **0 missing**.
Verified by `missingRoutes(routes, loadFixtures("fixtures"))`.

| Module dir | Fixtures |
|---|---|
| auth | 26 |
| account | 14 |
| admin_core | 13 |
| admin_crud | 18 |
| admin_reporting | 11 |
| admin_triage | 19 |
| payment_public | 9 |
| public_misc | 11 |
| rewrite | 8 |

Excluded from the gate (documented in routes.txt): `POST /v1/rewrite` —
go-only streaming path, Node 404s.

Two harness correctness fixes landed this phase, both required before the full
gate means anything:
- `loadFixtures` now recurses sub-dirs (`filepath.WalkDir`). The old
  non-recursive `Glob(dir/*.json)` matched **zero** files when run-gate.sh
  points at the top-level `fixtures/` dir → the gate would have passed
  vacuously. (commit 7cf8fc29)
- `raw_body` fixture escape hatch added so the one multipart route
  (`POST /bug-reports`) can be exercised with a real multipart payload — the
  Go handler `ParseMultipartForm`s first, so a JSON body would 400 on Go but
  201 on Node. (commit 0018ad2b)

## Live gate — RUN ON THE VPS (not runnable on the dev box)

Requires the docker shadow stack: Node backend :3200 + Go backend :3201 +
frozen template DB. The dev machine has no shadow stack, so only the static
coverage check runs here. Run on the VPS:

```bash
cd /opt/draftright/backend-rewrite-go   # or wherever the repo is checked out
./deploy/shadow/run-gate.sh --env-file /opt/draftright/.env.dev
```

The gate: builds the frozen template once (`augment.sql`), brings up both
backends, then for each of the 129 fixtures resets both DBs from the template,
replays the request against Node and Go, and diffs status + body. Any diff =
FAIL with a per-fixture report.

### If a fixture FAILS

STOP. Do **not** flip Caddy. Triage the diff:
- **Fixture bug** (wrong ignore_value_of, stale seeded id, env-gated value) →
  fix the fixture, re-run.
- **Real Go ≠ Node divergence** → Node is the authority. Do NOT patch Go on
  this fixture branch. File a parity issue and fix on a separate branch off
  develop, then re-run the gate.

### Watch-items for the first live run (flagged during fixture authoring)

| Item | Why watch | Resolution |
|---|---|---|
| `subscriptions_grant` | Go `GrantedSub` view omits `store_transaction_id`; if Node's TypeORM `save()` returns it as null the gate flags a key-presence diff. Highest-risk item. | Confirm Node grant response keys vs Go on first run. |
| `errors_suggest_fix`, `bug_reports_fix_proposal` | Call the LLM (seeded Ollama). If Ollama is unreachable in the shadow stack both backends take an error path — parity of the error envelope is then under test, not the happy path. | Ensure Ollama reachable, or accept the error-path parity check. |
| `admin/ai-providers` create `temperature` | Confirmed real divergence (logged for issue-filing). | See parity issues. |
| `admin/settings/test-email` valid recipient | Env-gated (SMTP creds). | Verify env in the shadow stack. |
| malformed-JSON body | Node body-parser SyntaxError → 500; Go path may differ. Pre-existing gap. | See parity issues. |
| `admin/analytics` month buckets | TZ boundary; mitigated by mid-month frozen timestamp. | Confirm no diff at month edges. |

## After the gate is green

1. Record the green run here (date, commit, fixture count, 0 diffs).
2. Flip Caddy: swap the whole upstream from the Node port to the Go port for
   the API host. Keep Node running for instant rollback.
3. Health-check, watch logs/metrics. Roll back by reverting the Caddy upstream
   if anything regresses.
4. Decommission Node once stable.

## Green-run log

_(empty — awaiting first VPS run)_
