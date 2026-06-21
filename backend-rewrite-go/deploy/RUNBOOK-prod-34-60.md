# Prod deploy runbook — Go port release: #34 + #60

**Release scope:** `origin/main..origin/develop` = exactly two batches, 9 commits.

| Issue | What | Parity authority | Gate-coverable? |
|---|---|---|---|
| **#34** | Route `ollama`-type providers through the OpenAI wire in the completer factory; send `Authorization` header only when api_key set | `OpenAiStrategy` (Node) | **No** — gate stubs the AI provider |
| **#60** | Build `openai.Client.Complete` request body byte-identical to Node (`temperature` / gpt-5 `reasoning_effort` / ollama `isLocal` layout / drop `stream` key) | `OpenAiStrategy.buildBody` (Node) | **No** — same reason |

Both changes are **invisible to the shadow gate** (gate uses a deterministic memory provider, so the real upstream request body/wire is never exercised). They are verified by Go unit tests + reading the Node authority. The gate's job here is to prove the **HTTP envelope of `/rewrite` is unchanged** — i.e. no parity regression introduced by the surrounding edits.

---

## Pre-flight (already done at authoring)

- [x] `git fetch origin`; `origin/develop` = `708a9abf`, local develop synced.
- [x] Release delta confirmed = #34 + #60 only (no stray commits). Everything else already in main.
- [x] **Local Go gate green on develop:** `gofmt -l .` empty · `go vet ./...` clean · `go build ./...` clean · `go test ./... -race` exit 0.
- [x] **No GitHub CI gate applies** — `backend-ci.yml` is path-filtered to `backend/**`; the Go port lives in `backend-rewrite-go/**`. There is intentionally no Actions run for this push. Do NOT wait on one.

## Step 1 — Live shadow gate 148/148 (dev VPS) — **needs user go-ahead**

Run ON the dev VPS from a `develop` checkout. Touches only `draftright_shadow_*` DBs; never `draftright_dev`/prod.

```bash
# on dev VPS, in backend-rewrite-go checkout at origin/develop (708a9abf)
cd <repo>/backend-rewrite-go
git fetch origin && git checkout 708a9abf
# .env.shadow must hold JWT_SECRET + JWT_REFRESH_SECRET + NODE_DATABASE_URL
#   + GO_DATABASE_URL + MAINT_DSN  (never echo MAINT_DSN; never `pgrep -af shadowdiff`)
bash deploy/shadow/run-gate.sh
```

**Pass condition:** `148/148`. Anything less = STOP, do not flip.

Why run it even though #34/#60 are AI-stubbed-invisible: it confirms the edits didn't perturb the `/rewrite` envelope, auth, or any other route. It is the standing "ship nothing to prod without the live shadow gate passing" rule (`backend-rewrite-go/CLAUDE.md`).

> This is a remote VPS op against prod-ish secrets (`.env.shadow`, `MAINT_DSN`). Execute only on explicit user go-ahead.

## Step 2 — Merge develop → main

```bash
cd /opt/openAi/DraftRight
git fetch origin
git checkout main && git pull --ff-only
git merge develop --no-ff -m "Release: Go port #34 (ollama→openai wire) + #60 (openai body parity)"
git push origin main
```

GitFlow: `--no-ff`, never direct-commit. After this, `main` carries #34 + #60.

## Step 3 — Rebuild + redeploy Go backend on prod — **needs user go-ahead**

Per infra memory: prod api.draftright.info is already Go (`draftright-backend-go-1`, :3001). Redeploy = rebuild image from `main` + recreate the one container. Compose project at `/opt/draftright`.

```bash
# on PROD VPS (deploy@ alias `draftright`; deploy is in docker group)
cd /opt/draftright
git fetch origin && git checkout main && git pull --ff-only
# rebuild the Go image tag the compose file references
docker build -t draftright-backend-go:latest backend-rewrite-go
docker compose up -d backend-go
```

No DB migration in this release (body/wire-only changes; no schema touch). Skip the migrate step.

## Step 4 — Post-deploy health check

```bash
docker ps --filter name=backend-go --format '{{.Names}}\t{{.Status}}'   # healthy/up
docker logs --tail=50 draftright-backend-go-1                            # no boot errors
curl -fsS https://api.draftright.info/health || echo "HEALTH FAIL"
# real-wire smoke (the thing the gate can't cover): one /rewrite call,
# confirm 200 + sane rewritten text (exercises the #34/#60 body path live)
```

Watch specifically: an `ollama`-type provider row (if any admin-created one exists) must carry an OpenAI-compatible `endpoint_url` — #34 now POSTs the OpenAI shape to it. Default seed has only an OpenAI provider, so stock deploys are unaffected.

## Step 5 — Issue lifecycle

- Apply `status: deployed to production` + comment (what shipped, what verified) on **#34** and **#60**.
- Add a `## ✅ How to Verify` comment per issue (URL + what changed + steps), self-tested on prod first.
- **Do NOT close** — only Tan/Mark close after manual verification.

## Rollback

Node image is kept rollback-ready (memory: "Node stopped/rollback-ready"). If the live `/rewrite` smoke regresses:

```bash
cd /opt/draftright
git checkout <prev-main-sha> -- .          # or revert the merge
docker build -t draftright-backend-go:latest backend-rewrite-go && docker compose up -d backend-go
# or, last resort, bring the Node container back per infra memory
```

Body/wire change on a single container → fast revert, no schema to unwind.

---

**Gate order, hard rules:** local Go gate ✅ (done) → live shadow 148/148 (Step 1, on go-ahead) → merge main → rebuild+redeploy (Step 3, on go-ahead) → health → labels. Never flip on a sub-148 gate. Prod steps need explicit per-batch authorization.
