#!/usr/bin/env bash
# DraftRight — sync a gitignored deploy .env between this checkout and a server.
#
# The prod/dev .env files are gitignored, so the deploy's `git pull` never
# carries them: env changes are edited by hand on the box and there is no other
# record of what a server actually holds. That gap is exactly how a secret can
# sit empty in prod while everyone assumes it was set — the two copies drift and
# nothing shows it. This tool makes the drift visible (by key + value hash,
# never the value itself) and lets you push/pull deliberately.
#
# Two targets, because DraftRight has two env files in two places:
#   prod  ->  deploy@:/opt/draftright/.env                     (root-owned; writes via sudo)
#   dev   ->  deploy@:/home/deploy/deploys/draftright-dev/.env.dev
# Local canonical copies live beside this script: deploy/.env.prod, deploy/.env.dev
# (both gitignored). Root SSH on the droplet is LOCKED — everything goes through
# the `deploy` user (ssh alias `draftright`), which has passwordless sudo.
#
# Usage:
#   ./env-sync.sh prod diff                   # which KEYS differ (never prints values)
#   ./env-sync.sh prod push                   # local  -> server (backs up the remote first)
#   ./env-sync.sh prod pull                   # server -> local  (backs up the local first)
#   ./env-sync.sh prod push --restart backend # push, then recreate those services
#   ./env-sync.sh dev  push --yes             # skip the confirmation prompt
#
# Values are never printed by any subcommand — only key names and an 8-char hash
# of each value, so a drifting secret is visible without being disclosed.

set -euo pipefail

SERVER="draftright"                 # ssh alias = deploy@129.212.208.248 (root is locked)
HERE="$(cd "$(dirname "$0")" && pwd)"

# --- Parse args: <target> <cmd> [opts] --------------------------------------
TARGET="${1:-}"; shift || true
CMD="${1:-diff}"; shift || true
ASSUME_YES=false
RESTART=()
while [ $# -gt 0 ]; do
  case "$1" in
    --yes|-y) ASSUME_YES=true ;;
    --restart) shift; while [ $# -gt 0 ] && [ "${1#--}" = "$1" ]; do RESTART+=("$1"); shift; done; continue ;;
    *) echo "unknown option: $1" >&2; exit 2 ;;
  esac
  shift
done

# --- Target config ----------------------------------------------------------
# REMOTE_ENV : path of the .env on the server
# LOCAL_ENV  : local canonical copy (gitignored)
# REMOTE_SUDO: "sudo" when the remote file needs root to write, else ""
# COMPOSE    : how to run docker compose for --restart, from REMOTE_CWD
case "$TARGET" in
  prod)
    REMOTE_ENV="/opt/draftright/.env"
    LOCAL_ENV="$HERE/.env.prod"
    REMOTE_SUDO="sudo"                                    # root-owned file
    REMOTE_CWD="/opt/draftright"
    COMPOSE="docker compose -f docker-compose.prod.yml"
    ;;
  dev)
    REMOTE_ENV="/home/deploy/deploys/draftright-dev/.env.dev"
    LOCAL_ENV="$HERE/.env.dev"
    REMOTE_SUDO=""                                        # deploy owns it
    REMOTE_CWD="/home/deploy/deploys/draftright-dev"
    COMPOSE="docker compose -f docker-compose.yml -f docker-compose.dev.yml"
    ;;
  *)
    echo "usage: $0 {prod|dev} {diff|push|pull} [--yes] [--restart svc...]" >&2
    exit 2
    ;;
esac

# sha of each value, so drift is visible without disclosing secrets. Uses
# whichever hasher the platform has (macOS ships shasum, Linux sha256sum).
hash_cmd() {
  if command -v sha256sum >/dev/null 2>&1; then echo "sha256sum"; else echo "shasum -a 256"; fi
}

# KEY <8-char hash of value>, sorted. Blank lines and comments ignored.
# Tolerant of a missing/empty file (a fresh pull has no local copy yet, and
# grep's exit-1/2 must not trip `set -e`).
fingerprint_local() {
  [ -f "$LOCAL_ENV" ] || return 0
  local h; h="$(hash_cmd)"
  { grep -E '^[A-Za-z_][A-Za-z0-9_]*=' "$LOCAL_ENV" 2>/dev/null || true; } | sort | while IFS= read -r line; do
    printf '%s %s\n' "${line%%=*}" "$(printf %s "${line#*=}" | $h | cut -c1-8)"
  done
}

# The remote file is world-readable (prod is 644), so reading needs no sudo.
fingerprint_remote() {
  ssh "$SERVER" "{ grep -E '^[A-Za-z_][A-Za-z0-9_]*=' '$REMOTE_ENV' 2>/dev/null || true; } | sort | while IFS= read -r line; do
      printf '%s %s\n' \"\${line%%=*}\" \"\$(printf %s \"\${line#*=}\" | sha256sum | cut -c1-8)\"
    done"
}

require_local() {
  [ -f "$LOCAL_ENV" ] || { echo "no local env at $LOCAL_ENV — run '$0 $TARGET pull' first to seed it" >&2; exit 1; }
}

confirm() {
  $ASSUME_YES && return 0
  printf '%s [y/N] ' "$1"
  read -r reply
  case "$reply" in y|Y|yes|YES) return 0 ;; *) echo "aborted."; exit 1 ;; esac
}

show_diff() {
  local l r
  l="$(fingerprint_local)"
  r="$(fingerprint_remote)"

  echo "→ Keys only in LOCAL (would be added to the server):"
  comm -23 <(echo "$l" | cut -d' ' -f1) <(echo "$r" | cut -d' ' -f1) | sed 's/^/    + /' || true
  echo "→ Keys only on SERVER (would be LOST by a push):"
  comm -13 <(echo "$l" | cut -d' ' -f1) <(echo "$r" | cut -d' ' -f1) | sed 's/^/    - /' || true
  echo "→ Keys whose VALUE differs:"
  comm -3 <(echo "$l") <(echo "$r") | awk '{print $1}' | sort -u \
    | comm -12 - <(comm -12 <(echo "$l" | cut -d' ' -f1) <(echo "$r" | cut -d' ' -f1)) \
    | sed 's/^/    ~ /' || true
  echo "  (values are never shown — compare by hand on the box if you need one)"
}

case "$CMD" in
  diff)
    require_local
    show_diff
    ;;

  push)
    require_local
    echo "target: $TARGET  ($SERVER:$REMOTE_ENV)"
    show_diff
    echo ""
    # A push replaces the remote file wholesale, so a key that exists only on
    # the server is destroyed. The diff above is the last chance to notice.
    confirm "Overwrite $SERVER:$REMOTE_ENV with the local copy?"
    stamp="$(date +%Y%m%d-%H%M%S)"
    echo "→ Backing up remote to .env.bak-$stamp"
    ssh "$SERVER" "cd '$REMOTE_CWD' && [ -f '$REMOTE_ENV' ] && $REMOTE_SUDO cp '$REMOTE_ENV' '$REMOTE_ENV.bak-$stamp' || true"
    echo "→ Pushing local env"
    # Can't scp straight onto a root-owned file as the deploy user, so land it
    # in /tmp and move it into place with the same privilege that owns it.
    tmp="/tmp/draftright-env-$stamp.$$"
    scp -q "$LOCAL_ENV" "$SERVER:$tmp"
    # cp onto the existing file keeps its owner/mode; then lock it down.
    ssh "$SERVER" "$REMOTE_SUDO cp '$tmp' '$REMOTE_ENV' && $REMOTE_SUDO chmod 600 '$REMOTE_ENV' && rm -f '$tmp'"
    echo "→ Verifying"
    show_diff
    if [ ${#RESTART[@]} -gt 0 ]; then
      echo "→ Recreating: ${RESTART[*]}"
      # --force-recreate: compose does NOT restart a container just because the
      # env file changed, so without this the new values are never picked up.
      # deploy is in the docker group → docker needs no sudo.
      ssh "$SERVER" "cd '$REMOTE_CWD' && $COMPOSE up -d --no-build --force-recreate ${RESTART[*]}"
      ssh "$SERVER" "cd '$REMOTE_CWD' && $COMPOSE ps --format 'table {{.Name}}\t{{.Status}}' | grep -E '$(IFS='|'; echo "${RESTART[*]}")|NAME'"
    fi
    ;;

  pull)
    echo "target: $TARGET  ($SERVER:$REMOTE_ENV)"
    show_diff
    echo ""
    confirm "Overwrite local $LOCAL_ENV with the server's copy?"
    if [ -f "$LOCAL_ENV" ]; then
      stamp="$(date +%Y%m%d-%H%M%S)"
      echo "→ Backing up local to .env.bak-$stamp"
      cp "$LOCAL_ENV" "$LOCAL_ENV.bak-$stamp"
    fi
    echo "→ Pulling server env"
    scp -q "$SERVER:$REMOTE_ENV" "$LOCAL_ENV"
    chmod 600 "$LOCAL_ENV"
    echo "→ Done. Remember: $(basename "$LOCAL_ENV") is gitignored and stays that way."
    ;;

  *)
    echo "usage: $0 {prod|dev} {diff|push|pull} [--yes] [--restart svc...]" >&2
    exit 2
    ;;
esac

echo ""
echo "=== env-sync ($TARGET/$CMD) complete ==="
