# Release & Auto-Update Runbook

How to ship a new desktop version so existing installs **detect → download →
install it automatically**. This is the exact process used for every recent
Windows/macOS release.

> **Source of truth.** This doc is *process*. The concrete values — droplet
> paths, DB table, download-URL filename conventions, the SQL upsert — live in
> the scripts and the workflow, and this doc deliberately does **not** restate
> them (a second copy would drift; that exact failure is issue #22). Where a
> value matters, the owning file is named. Read it there.

---

## The two halves (both already exist)

Auto-update is not something you "turn on" per release. Two pieces are
permanent; you only ever perform a **release**, and the pieces do the rest.

1. **Client** — `DraftRightWindows/.../Services/UpdateService.cs` is compiled
   into every build. It polls `/updates/latest` (~every 24 h, and via Settings →
   Check for updates), silently stages the installer to `%TEMP%`,
   **sha256-verifies** it (#22), prompts once, then launches it. Nothing to set
   up per user. (MSIX/Store installs use `StoreUpdateService` instead and update
   through the Store.)
2. **Server** — `GET /updates/latest` on the prod droplet, backed by the
   `app_releases` table. A release just writes a new row.

So "set up auto-update" = **cut a release** that writes the `app_releases` row.

---

## One command

```
scripts/release.sh <windows|macos> <version> [--yes] [--dry-run]
```

Orchestrates the whole flow below by driving the same scripts/workflow — it is
not a second release path. From a clean `develop`, with the `## <version>`
CHANGELOG section already written, it: bumps the version file, commits, merges
`--no-ff` into develop then main, pushes, and publishes:

- **windows** — tags `vX.Y.Z`, which triggers `build-windows.yml` to build +
  publish; then polls `/updates/latest` until it reports the new version.
- **macos** — builds the notarized universal DMG *first* (so a notarization
  failure aborts before any git push), then merges/pushes, tags `macos-vX.Y.Z`
  (never `vX.Y.Z` — that is the Windows trigger), runs `release-publish.sh`, and
  polls.

`--dry-run` prints every step and changes nothing. Preconditions (clean tree, on
`develop`, in sync with origin, CHANGELOG notes present, not already live) are
checked up front and fail fast. The manual steps below are what this automates —
read them to understand what the command does.

## Release runbook — Windows (tag-triggered, fully automated)

Windows publishes itself from a tag push. You do steps 1–4; CI does the rest.

```
1. Bump  <Version>  in
     DraftRightWindows/DraftRightWindows/DraftRightWindows.csproj

2. Add a section to  CHANGELOG.md  (repo root):
     ## X.Y.Z — YYYY-MM-DD
     ### Windows
     - user-facing bullet
   MANDATORY — the release job fails at the notes step if the "## X.Y.Z"
   section is absent (scripts/changelog-extract.sh). Windows-only notes go
   under "### Windows"; cross-cutting notes under "### All platforms".

3. Commit on a release branch, then:
     git merge --no-ff  <release-branch>   # into develop
     git merge --no-ff  develop            # into main
   (Follow GitFlow — never commit straight to develop/main. Let the develop
    build go green first: it is the real Windows compile of your change.)

4. Tag and push — THIS is the trigger:
     git tag -a vX.Y.Z -m "DraftRight Windows X.Y.Z"
     git push origin main vX.Y.Z
```

### What CI does automatically on the `v*` tag

`.github/workflows/build-windows.yml`:

- builds **x64 + arm64**, wraps each in an Inno Setup installer
  (`DraftRightWindows/installer/draftright.iss`),
- signs them **if** a cert is configured (dormant today — see Signing below),
- runs the **`publish-to-update-server`** job — fires *only* on a `v*` tag or a
  GitHub Release, never on a routine branch push. It scp's the **x64** installer
  to the droplet, upserts `app_releases` via `psql` using the shared statement
  in `scripts/app-release-upsert-sql.sh`, then verifies `/updates/latest`
  reports the new version **and** a matching non-empty sha256 (#22).

Existing installs pick it up on their next poll (or Check-now).

---

## Release runbook — macOS / Android / Linux (scripted)

These are not tag-automated; publish with the one script after building the
artifact. It owns upload + the `app_releases` row in one shot — that row is the
single source of truth for versions, driving both `/updates/latest` (in-app
updater) and the website download cards (read at build time):

```
scripts/release-publish.sh <platform> <version> <local-file> [--meta "…"]
   platforms: android | ios-sim | macos | windows | linux
```

- **macOS** additionally requires notarize + staple on the `.dmg` *before*
  publishing (Gatekeeper equivalent of the Windows signing story). Build/notarize
  first, then `release-publish.sh macos X.Y.Z <notarized.dmg>`.
- Requires the `draftright` ssh alias (see `reference_vps_ssh_admin_access`).
- Filename conventions per platform are defined in that script's header — do not
  hand-name the remote file.

`release-publish.sh` and the Windows CI job intentionally share
`scripts/app-release-upsert-sql.sh` for the DB write, so the two publish paths
cannot drift (they did once — #22, Windows shipped with an empty sha256 for two
months).

---

## Verify a release actually landed

CI already asserts this for Windows, but to check by hand:

```
curl -s "https://api.draftright.info/updates/latest?platform=windows" | jq .platforms.windows
scripts/verify-published-artifact.sh <download-url> <expected-bytes> --sha256 <hex> --deep
```

Why `verify-published-artifact.sh` and not a plain `curl -I`: a **missing** file
under `/downloads` does not 404 — Caddy falls through to the marketing SPA and
returns its HTML with `200`, so a status-only check passes against an absent
artifact (#144). The script checks size + hash.

---

## Gotchas (each fails silently)

- **No `## X.Y.Z` CHANGELOG section** → release job aborts at the notes step.
- **Only x64 publishes** to the update server; arm64 is build-only today.
- **`publish-to-update-server` gate** — only a `v*` tag or a GitHub Release
  triggers it. A branch push builds but publishes nothing.
- **macOS is a separate path** — a Windows tag does not ship macOS. Run
  `release-publish.sh macos …` for the DMG.

---

## The one blocker for Windows install to actually run — signing (#154)

Everything above **lands** the version. But the Windows installer is currently
**unsigned**, so Smart App Control / WDAC block the *launch* at the final step —
auto-update completes only for users who have Smart App Control off.

The signing is already **wired and dormant** in CI
(`DraftRightWindows/installer/sign-file.ps1`, called from `build-windows.yml`).
To make auto-update work for everyone, no code change is needed — provision a
cert and set the secrets:

- `WINDOWS_SIGNING_PFX_BASE64` + `WINDOWS_SIGNING_PFX_PASSWORD` (an OV/EV .pfx),
  **or** swap the PFX branch for Azure Trusted Signing (~$10/mo) per the script
  header.

Then the *same runbook* ships a signed build and the install step stops being
blocked. Until then, releases still reach only SAC-off users.

---

## File index (the actual owners)

| Concern | File |
|---|---|
| Client update logic | `DraftRightWindows/DraftRightWindows/Services/UpdateService.cs` |
| Windows build + publish | `.github/workflows/build-windows.yml` |
| Installer definition | `DraftRightWindows/installer/draftright.iss` |
| Installer signing (dormant) | `DraftRightWindows/installer/sign-file.ps1` |
| mac/Android/Linux publish | `scripts/release-publish.sh` |
| Shared DB upsert | `scripts/app-release-upsert-sql.sh` |
| Release-notes extraction | `scripts/changelog-extract.sh` |
| Publish verification | `scripts/verify-published-artifact.sh` |
| Release notes | `CHANGELOG.md` |
