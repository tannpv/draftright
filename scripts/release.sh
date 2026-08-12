#!/usr/bin/env bash
# One-command release: cut a new desktop version and make it available for
# download + auto-update.
#
#   scripts/release.sh <platform> <version> [--yes] [--dry-run]
#     platform : windows | macos
#     version  : semver, e.g. 2.3.42
#     --yes    : skip the confirmation prompt (for scripted use)
#     --dry-run: print every step, change nothing
#
# What it does (per platform), driving the SAME pieces a manual release uses —
# this script is an orchestrator, not a second release path:
#
#   windows : bump .csproj <Version> -> commit on a release branch -> merge
#             --no-ff into develop then main -> push -> tag vX.Y.Z. The tag is
#             the trigger: .github/workflows/build-windows.yml builds x64+arm64,
#             signs (when a cert is configured, #154), and its
#             publish-to-update-server job writes app_releases. We then poll
#             /updates/latest until it reports the new version.
#
#   macos   : bump Info.plist CFBundleShortVersionString (+ CFBundleVersion) ->
#             commit -> merge --no-ff into develop then main -> push ->
#             build-macos-dmg.sh (universal, Developer ID signed, notarized,
#             stapled) -> release-publish.sh macos (upload + app_releases +
#             versions.json) -> verify-published-artifact.sh + poll.
#             It tags `macos-vX.Y.Z`, NOT `vX.Y.Z`, on purpose: a `v*` tag is
#             the Windows CI trigger, so a macOS release must never push one.
#
# Preconditions (checked up front, fail fast):
#   - run from a clean git tree, on `develop`, in sync with origin/develop
#   - CHANGELOG.md has a "## <version>" section with notes for this platform
#     (notes are user-facing and must be human-written — this script will not
#      invent them). Uses scripts/changelog-extract.sh, same as the pipeline.
#   - macOS only: notarization credentials available to build-macos-dmg.sh.
#
# Requires: git, gh (Windows poll is optional), the ssh alias `draftright`
# (macOS publish), jq, curl.

set -euo pipefail

cd "$(dirname "$0")/.."
ROOT="$(pwd)"

# ── Constants (one source of truth) ────────────────────────────────────────
API_BASE="https://api.draftright.info"
CSPROJ="DraftRightWindows/DraftRightWindows/DraftRightWindows.csproj"
MAC_PLIST="DraftRight/Info.plist"
CHANGELOG="CHANGELOG.md"
BASE_BRANCH="develop"
RELEASE_BRANCH="main"
POLL_TRIES=60          # /updates/latest poll attempts
POLL_INTERVAL=15       # seconds between polls

# ── Args ────────────────────────────────────────────────────────────────────
PLATFORM="${1:-}"
VERSION="${2:-}"
shift 2 2>/dev/null || true
ASSUME_YES=0
DRY_RUN=0
for arg in "$@"; do
  case "$arg" in
    --yes)     ASSUME_YES=1 ;;
    --dry-run) DRY_RUN=1 ;;
    *) echo "Unknown option: $arg" >&2; exit 2 ;;
  esac
done

usage() { echo "Usage: $0 <windows|macos> <version> [--yes] [--dry-run]" >&2; exit 2; }
[ -n "$PLATFORM" ] && [ -n "$VERSION" ] || usage
case "$PLATFORM" in windows|macos) ;; *) echo "platform must be windows|macos" >&2; usage ;; esac
if ! [[ "$VERSION" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
  echo "version must be semver MAJOR.MINOR.PATCH (got '$VERSION')" >&2; exit 2
fi

say()  { printf '\033[1;36m==>\033[0m %s\n' "$*"; }
run()  { if [ "$DRY_RUN" = 1 ]; then printf '   [dry-run] %s\n' "$*"; else eval "$@"; fi; }

# ── Preconditions ───────────────────────────────────────────────────────────
say "Checking preconditions for $PLATFORM $VERSION"

if [ -n "$(git status --porcelain)" ]; then
  echo "Working tree is not clean — commit or stash first." >&2; exit 1
fi

git fetch origin --quiet
CURRENT_BRANCH="$(git branch --show-current)"
if [ "$CURRENT_BRANCH" != "$BASE_BRANCH" ]; then
  echo "Must be on '$BASE_BRANCH' (currently on '$CURRENT_BRANCH')." >&2; exit 1
fi
if [ "$(git rev-parse "$BASE_BRANCH")" != "$(git rev-parse "origin/$BASE_BRANCH")" ]; then
  echo "Local '$BASE_BRANCH' is not in sync with origin — pull/push first." >&2; exit 1
fi

# CHANGELOG notes must exist for this platform (same gate the pipeline enforces).
if ! NOTES="$(bash scripts/changelog-extract.sh "$VERSION" "$PLATFORM" 2>/dev/null)" || [ -z "$NOTES" ]; then
  echo "No '## $VERSION' notes for $PLATFORM in $CHANGELOG." >&2
  echo "Add a '## $VERSION — <date>' section with a '### Windows'/'### macOS' (or '### All platforms') bullet, then re-run." >&2
  exit 1
fi
say "Release notes found:"; printf '%s\n' "$NOTES" | sed 's/^/    /'

# Guard against re-releasing a version already live.
LIVE="$(curl -fsS "$API_BASE/updates/latest?platform=$PLATFORM" 2>/dev/null | jq -r ".platforms.$PLATFORM.version // \"\"" || true)"
if [ "$LIVE" = "$VERSION" ]; then
  echo "/updates/latest already serves $PLATFORM $VERSION — nothing to do." >&2; exit 1
fi
say "Currently live ($PLATFORM): ${LIVE:-unknown}  ->  releasing: $VERSION"

# ── Confirm ─────────────────────────────────────────────────────────────────
if [ "$ASSUME_YES" != 1 ] && [ "$DRY_RUN" != 1 ]; then
  echo
  echo "This will merge to '$RELEASE_BRANCH' and PUBLISH $PLATFORM $VERSION to production."
  read -r -p "Proceed? [y/N] " reply
  case "$reply" in y|Y|yes|YES) ;; *) echo "Aborted."; exit 0 ;; esac
fi

# ── Shared git release-commit + merge ───────────────────────────────────────
# Bumps the version file (callback), commits on a release branch, merges
# --no-ff into develop then main, pushes both. Leaves you on develop.
release_branch_merge() {
  local prefix="$1"        # e.g. chore/windows
  local msg="$2"           # commit subject
  local branch="${prefix}-${VERSION}-$(git rev-parse --short HEAD)"

  run "git checkout -b '$branch'"
  run "git commit -aq -m '$msg'"
  run "git checkout '$BASE_BRANCH'"
  run "git merge --no-ff '$branch' -m 'Merge $branch: release $VERSION'"
  run "git checkout '$RELEASE_BRANCH'"
  run "git merge --no-ff '$BASE_BRANCH' -m 'Merge $BASE_BRANCH -> $RELEASE_BRANCH: $PLATFORM $VERSION'"
  run "git push origin '$BASE_BRANCH' '$RELEASE_BRANCH'"
  run "git checkout '$BASE_BRANCH'"
}

# ── Poll /updates/latest until it reports VERSION ───────────────────────────
poll_updates() {
  [ "$DRY_RUN" = 1 ] && { say "[dry-run] would poll $API_BASE/updates/latest?platform=$PLATFORM for $VERSION"; return 0; }
  say "Waiting for /updates/latest to report $PLATFORM $VERSION"
  local i got
  for ((i=1; i<=POLL_TRIES; i++)); do
    got="$(curl -fsS "$API_BASE/updates/latest?platform=$PLATFORM" 2>/dev/null | jq -r ".platforms.$PLATFORM.version // \"\"" || true)"
    if [ "$got" = "$VERSION" ]; then say "Live: $PLATFORM $VERSION"; return 0; fi
    printf '   [%d/%d] currently %s …\n' "$i" "$POLL_TRIES" "${got:-none}"
    sleep "$POLL_INTERVAL"
  done
  echo "Timed out waiting for $PLATFORM $VERSION on /updates/latest." >&2
  return 1
}

# ── Windows ─────────────────────────────────────────────────────────────────
release_windows() {
  say "Bumping $CSPROJ <Version> -> $VERSION"
  run "/usr/bin/sed -i '' 's|<Version>[^<]*</Version>|<Version>$VERSION</Version>|' '$CSPROJ'"
  release_branch_merge "chore/windows" "chore(windows): release $VERSION"

  say "Tagging v$VERSION (triggers build-windows.yml build + publish)"
  run "git tag -a 'v$VERSION' -m 'DraftRight Windows $VERSION'"
  run "git push origin 'v$VERSION'"

  poll_updates
  say "Windows $VERSION published. CI verified the sha256 as part of publish (#22)."
}

# ── macOS ───────────────────────────────────────────────────────────────────
release_macos() {
  say "Bumping $MAC_PLIST CFBundleShortVersionString -> $VERSION"
  run "/usr/libexec/PlistBuddy -c 'Set :CFBundleShortVersionString $VERSION' '$MAC_PLIST'"
  # CFBundleVersion is a monotonic build number; bump it if it is an integer.
  local build
  build="$(/usr/libexec/PlistBuddy -c 'Print :CFBundleVersion' "$MAC_PLIST" 2>/dev/null || echo 0)"
  if [[ "$build" =~ ^[0-9]+$ ]]; then
    run "/usr/libexec/PlistBuddy -c 'Set :CFBundleVersion $((build+1))' '$MAC_PLIST'"
  fi

  # Build + notarize FIRST, from the (uncommitted) bumped plist, so a build or
  # notarization failure aborts BEFORE any git merge/push/tag — never leaving a
  # pushed version with no published artifact. build-macos-dmg.sh reads the
  # plist on disk, so the uncommitted bump is picked up.
  say "Building notarized universal DMG (build-macos-dmg.sh)"
  run "bash scripts/build-macos-dmg.sh"
  local dmg="dist/DraftRight-${VERSION}.dmg"
  if [ "$DRY_RUN" != 1 ] && [ ! -f "$dmg" ]; then
    echo "Expected $dmg not produced by build-macos-dmg.sh." >&2; exit 1
  fi

  # DMG in hand — now land the version in git and publish it.
  release_branch_merge "chore/macos" "chore(macos): release $VERSION"

  # Provenance tag — deliberately NOT 'v*' so it never triggers the Windows CI.
  say "Tagging macos-v$VERSION (provenance only; not a Windows trigger)"
  run "git tag -a 'macos-v$VERSION' -m 'DraftRight macOS $VERSION'"
  run "git push origin 'macos-v$VERSION'"

  say "Publishing DMG to the update server (release-publish.sh macos)"
  run "bash scripts/release-publish.sh macos '$VERSION' '$dmg'"

  poll_updates
  say "macOS $VERSION published."
}

# ── Go ──────────────────────────────────────────────────────────────────────
case "$PLATFORM" in
  windows) release_windows ;;
  macos)   release_macos ;;
esac

# Refresh the marketing site so its download section shows the new version.
# DownloadCTA fetches versions from /updates/latest at BUILD time, so the static
# page only updates on redeploy — do it here (now that poll_updates confirmed the
# new version is live) instead of relying on someone remembering. The Store link
# itself is stable and needs no change.
say "Refreshing the website download section (deploy-website.sh)"
run "bash scripts/deploy-website.sh"

say "Done. $PLATFORM $VERSION is live on /updates/latest, the website reflects it — existing installs auto-update on their next check."
