#!/usr/bin/env bash
# Deploy the marketing website to prod.
# CRITICAL: must NOT delete /var/www/draftright/downloads/ — those binary
# artifacts (DraftRight-Android-X.Y.Z.apk, .zip, .tar.gz) live alongside
# the static site but are not built by `npm run build`. Yesterday's deploy
# wiped them by accident; --exclude=downloads guards against that.
set -euo pipefail

cd "$(dirname "$0")/../website"
npm run build
rsync -av --delete --exclude=downloads dist/ draftright:/var/www/draftright/
echo "Deployed. Live URLs:"
for path in / /signup /verify-email /pricing /download /account; do
  printf "  https://draftright.info%-15s -> HTTP %s\n" "$path" "$(curl -sS -o /dev/null -w '%{http_code}' "https://draftright.info$path")"
done
