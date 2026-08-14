#!/usr/bin/env bash
# Build the DraftRight Linux source tarball that release-publish.sh uploads.
#
#   scripts/build-linux-tarball.sh [output-dir]     # default: ./dist
#
# Produces  DraftRight-Linux-<version>.tar.gz  where <version> is the single
# source of truth in DraftRightLinux/draftright/__version__.py — never a literal
# passed on the command line, so the artifact name can't drift from the app.
#
# Uses `git archive`, NOT `tar` of the working tree, on purpose: it packages
# only git-tracked files, so gitignored local material (the GCC/ Google OAuth
# client_secret, __pycache__, .egg-info, test caches) can never leak into a
# PUBLIC download — the reason this is a script and not a one-off tar command.
#
# The tarball extracts to a top-level DraftRight-Linux-<version>/ directory,
# matching the website's `tar -xzf DraftRight-Linux-*.tar.gz` instruction.
set -euo pipefail
cd "$(dirname "$0")/.."

PREFIX_SUBTREE="DraftRightLinux"
VERSION_FILE="$PREFIX_SUBTREE/draftright/__version__.py"

# Single source of truth for the version.
VERSION=$(sed -nE 's/^__version__[[:space:]]*=[[:space:]]*["'"'"']([^"'"'"']+)["'"'"'].*/\1/p' "$VERSION_FILE")
if [ -z "$VERSION" ]; then
  echo "ERROR: could not read __version__ from $VERSION_FILE" >&2
  exit 1
fi

OUT_DIR="${1:-dist}"
mkdir -p "$OUT_DIR"
NAME="DraftRight-Linux-${VERSION}.tar.gz"
OUT="$OUT_DIR/$NAME"

echo "==> Building $NAME from git-tracked $PREFIX_SUBTREE/ (version $VERSION)"

# Fail loudly if the working tree has uncommitted changes under the subtree —
# git archive packages the committed tree, so uncommitted edits would silently
# NOT ship. Better to know before publishing.
if ! git diff --quiet -- "$PREFIX_SUBTREE" || ! git diff --cached --quiet -- "$PREFIX_SUBTREE"; then
  echo "WARNING: uncommitted changes under $PREFIX_SUBTREE/ will NOT be in the tarball" >&2
  echo "         (git archive packages the committed tree). Commit first." >&2
fi

git archive --format=tar.gz \
  --prefix="DraftRight-Linux-${VERSION}/" \
  -o "$OUT" \
  HEAD:"$PREFIX_SUBTREE"

# Sanity: it must be a real gzip, not empty, and must NOT contain the secret.
if ! file "$OUT" | grep -qi gzip; then
  echo "ERROR: $OUT is not a gzip archive" >&2
  exit 1
fi
if tar tzf "$OUT" | grep -qiE "client_secret|\.pyc$|__pycache__"; then
  echo "ERROR: tarball contains excluded/secret material — aborting" >&2
  tar tzf "$OUT" | grep -iE "client_secret|__pycache__" >&2
  exit 1
fi

SIZE=$(command -v gstat >/dev/null 2>&1 && gstat -c%s "$OUT" || stat -f%z "$OUT" 2>/dev/null || stat -c%s "$OUT")
echo "==> Built $OUT ($((SIZE/1024)) KB, $(tar tzf "$OUT" | wc -l | tr -d ' ') files)"
echo "    Publish with: scripts/release-publish.sh linux $VERSION $OUT"
