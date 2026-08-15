#!/usr/bin/env bash
#
# publish-ime-pack.sh <pack-path> <meta-path>
#
# ONE source of truth for publishing an IME dictionary pack to the marketing
# webroot (draftright.info/ime-packs/), so clients can download it. Both
# build-jp-dict-pack.sh and build-zh-dict-pack.sh call this — the publish step
# used to be copy-pasted into each, which is exactly the drift RULE #1 forbids.
#
# Why the explicit chmod: `rsync -a` preserves the LOCAL file mode. A pack that
# lands 0600 is unreadable by Caddy, which then falls through to the SPA
# try_files and serves the HTML index (200!) instead of the bytes — silently
# un-publishing the pack. ja-v3 and zh-v1 both hit this. Force 0644.
#
# Why the served-sha verify: publishing "succeeded" (rsync exit 0) is not proof
# the URL serves the pack. Fetch it back and compare sha256 — a perm slip, a
# missing /ime-packs Caddy handle, or the HTML fallback all fail here loudly.
set -euo pipefail

PACK_PATH="${1:?usage: publish-ime-pack.sh <pack-path> <meta-path>}"
META_PATH="${2:?usage: publish-ime-pack.sh <pack-path> <meta-path>}"

SSH_HOST="${DEPLOY_HOST:-draftright}"
REMOTE_DIR="/var/www/draftright/ime-packs"
BASE_URL="https://draftright.info/ime-packs"

PACK_NAME="$(basename "$PACK_PATH")"
META_NAME="$(basename "$META_PATH")"

ssh "$SSH_HOST" "mkdir -p $REMOTE_DIR"
rsync -avz "$PACK_PATH" "$META_PATH" "$SSH_HOST:$REMOTE_DIR/"
ssh "$SSH_HOST" "chmod 644 '$REMOTE_DIR/$PACK_NAME' '$REMOTE_DIR/$META_NAME'"

EXPECT_SHA="$(shasum -a 256 "$PACK_PATH" | awk '{print $1}')"
SERVED_SHA="$(curl -fsSL "$BASE_URL/$PACK_NAME" | shasum -a 256 | awk '{print $1}')"
if [ "$SERVED_SHA" != "$EXPECT_SHA" ]; then
  echo "❌  Publish verify FAILED for $PACK_NAME" >&2
  echo "    served sha : $SERVED_SHA" >&2
  echo "    expected   : $EXPECT_SHA" >&2
  echo "    Likely the HTML fallback (missing file / wrong perms / no Caddy /ime-packs handle)." >&2
  exit 1
fi

echo "✅  Published + verified: $BASE_URL/$PACK_NAME (sha $EXPECT_SHA)"
