# Dev environment runbook

`dev.draftright.info` + `api.dev.draftright.info`. Same VPS as prod (DO
droplet, `129.212.208.248`), separate containers, separate DB.

## What it gets you

- A live `/payment/portal` endpoint to test the Manage button on
  every client.
- A live `/payment/success` Astro page so mobile Universal Link
  return-to-app can be verified end-to-end.
- A separate Postgres DB so test payments / users don't touch prod.
- A separate admin portal at `admin.dev.draftright.info` for
  granting Pro to test accounts without touching prod data.
- Same backend / admin / website code as `develop` branch; cut new
  builds from there.

## One-time setup (first time only)

```bash
# On the VPS
cd /opt/draftright
git fetch origin
git checkout develop
git pull --ff-only origin develop

# Build the new images
docker compose -f docker-compose.dev.yml build

# Create the dev DB inside the running prod Postgres container.
# `postgres-prod` is the container name in the existing compose
# file — adjust if yours differs.
docker exec -it $(docker compose ps -q postgres) \
    psql -U draftright -c 'CREATE DATABASE draftright_dev;'

# Add the Caddy block + reload
sudo cat deploy/Caddyfile.dev >> /etc/caddy/Caddyfile
sudo systemctl reload caddy
sudo journalctl -u caddy -n 50   # confirm cert obtained for the two new hosts

# Bring up the dev services alongside prod
docker compose --env-file .env.dev -f docker-compose.dev.yml up -d

# Seed the dev DB (admin user, free plan, default AI provider)
docker exec -it dr-backend-dev npx ts-node src/seed.ts
```

## Verify

```bash
curl -s https://api.dev.draftright.info/health | jq
# {
#   "app": "draftright",
#   "status": "ok",
#   ...
# }

curl -sI https://dev.draftright.info/.well-known/apple-app-site-association | grep -i content-type
# content-type: application/json
```

## Daily redeploy from develop

Every push to `develop` should redeploy dev so testing tracks the
branch:

```bash
cd /opt/draftright
git pull --ff-only origin develop
docker compose -f docker-compose.dev.yml build
docker compose --env-file .env.dev -f docker-compose.dev.yml up -d
docker compose ps                     # confirm `dr-backend-dev` is `running`
docker logs dr-backend-dev --tail 50  # spot any boot errors
```

A wrapper script could go in `scripts/deploy-dev.sh` later — for now
the four lines above are the entire flow.

## Mobile build → dev backend

The mobile app reads its backend URL from in-app
Settings → Advanced → Backend URL.  No rebuild needed:

1. Install the latest APK / IPA from `develop` (`flutter build apk
   --release`).
2. Open the app, go to Settings → Advanced.
3. Replace the URL with `https://api.dev.draftright.info`.
4. Re-sign-in (the JWT issued by prod isn't valid against dev).

Test:
- Subscription tab loads plan info
- Manage button works (LS / Stripe portal opens)
- Universal Link return — after LS checkout the app reopens at the
  Subscription screen (requires AASA hosted, see Caddy block above)

## Env vars

`.env.dev` lives next to `.env` on the VPS.  Copy from prod and
override only the bits that should differ.  Recommended overrides:

```
# Different signing secrets so dev JWTs can't authenticate against prod
JWT_SECRET_DEV=...generate fresh...
JWT_REFRESH_SECRET_DEV=...generate fresh...

# Lemon Squeezy test-mode keys (NOT prod keys)
LEMONSQUEEZY_API_KEY_DEV=ls_test_...
LEMONSQUEEZY_STORE_ID_DEV=...
LEMONSQUEEZY_PRO_VARIANT_ID_DEV=...
LEMONSQUEEZY_WEBHOOK_SECRET_DEV=...

# Stripe test-mode keys (NOT prod keys)
STRIPE_SECRET_KEY_DEV=sk_test_...
STRIPE_WEBHOOK_SECRET_DEV=whsec_test_...
```

If a `*_DEV` is unset, the dev backend falls back to the prod value
in the same `.env` — fine for local-only experiments but **never**
do this for payment keys (test purchases would hit prod merchant
accounts).

## Promote dev → prod

After verifying on dev:

```bash
# On the VPS
cd /opt/draftright
git checkout main
git merge --no-ff develop -m "Merge develop into main: <feature>"
git push origin main
docker compose pull   # if using a registry; otherwise:
docker compose build
docker compose up -d
docker compose ps     # all healthy
curl -s https://api.draftright.info/health | jq
```

## Tear down dev

```bash
docker compose --env-file .env.dev -f docker-compose.dev.yml down
docker exec -it $(docker compose ps -q postgres) \
    psql -U draftright -c 'DROP DATABASE IF EXISTS draftright_dev;'
# Remove the dev Caddy block from /etc/caddy/Caddyfile and reload.
```
