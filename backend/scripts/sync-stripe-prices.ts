/**
 * sync-stripe-prices.ts — one-shot script to populate plans.stripe_price_id
 * for every paid plan row.
 *
 * For each plan row with currency != NULL and price_cents > 0:
 *   1. Find or create a Stripe Product named "DraftRight {plan.name}".
 *   2. Find or create a Stripe Price for (product, currency, billing_period, amount).
 *   3. Update plan row with the returned price_id.
 *
 * Idempotent: re-running is a no-op if all rows already have stripe_price_id.
 *
 * Run from backend/:
 *   STRIPE_SECRET_KEY=sk_test_... \
 *   DATABASE_URL=postgres://draftright:...@... \
 *     npx ts-node scripts/sync-stripe-prices.ts
 *
 * On the droplet:
 *   sudo docker exec -it draftright-backend-1 \
 *     sh -c 'STRIPE_SECRET_KEY=$STRIPE_SECRET_KEY npx ts-node /app/scripts/sync-stripe-prices.ts'
 */

import 'reflect-metadata';
import { DataSource } from 'typeorm';
import Stripe from 'stripe';

interface PlanRow {
  id: string;
  name: string;
  daily_limit: number;
  price_cents: number;
  currency: string | null;
  stripe_price_id: string | null;
  trial_days: number;
  billing_period: string;
  is_active: boolean;
}

async function main() {
  const stripeKey = process.env.STRIPE_SECRET_KEY;
  if (!stripeKey) {
    console.error('STRIPE_SECRET_KEY env var is required.');
    process.exit(1);
  }

  const databaseUrl = process.env.DATABASE_URL;
  if (!databaseUrl) {
    console.error('DATABASE_URL env var is required.');
    process.exit(1);
  }

  const stripe = new Stripe(stripeKey);

  const ds = new DataSource({
    type: 'postgres',
    url: databaseUrl,
    synchronize: false,
    logging: false,
    entities: [],
  });
  await ds.initialize();

  console.log('Connected to DB. Loading paid plans...');

  const rows: PlanRow[] = await ds.query(`
    SELECT id, name, daily_limit, price_cents, currency, stripe_price_id, trial_days, billing_period, is_active
    FROM plans
    WHERE price_cents > 0 AND is_active = true
    ORDER BY name, currency, billing_period
  `);

  console.log(`Found ${rows.length} paid plan rows.\n`);

  // Cache: one Stripe Product per (plan name) — re-used across currencies.
  const productCache = new Map<string, string>();

  let updated = 0;
  let skipped = 0;

  for (const row of rows) {
    const label = `${row.name} ${row.currency} ${row.billing_period}`;

    if (row.stripe_price_id) {
      console.log(`✓ ${label}: already has price_id ${row.stripe_price_id} — skipping.`);
      skipped++;
      continue;
    }

    // ── Find or create Product ─────────────────────────────────
    let productId = productCache.get(row.name);
    if (!productId) {
      // Try to find an existing product by name (Stripe doesn't enforce unique names,
      // but we can look up via list+filter).
      const existing = await stripe.products.list({ active: true, limit: 100 });
      const found = existing.data.find((p) => p.name === `DraftRight ${row.name}`);
      if (found) {
        productId = found.id;
        console.log(`  reusing Product ${productId} for "${row.name}"`);
      } else {
        const created = await stripe.products.create({
          name: `DraftRight ${row.name}`,
          description: `${row.daily_limit === -1 ? 'Unlimited' : row.daily_limit} rewrites/day`,
          metadata: { plan_name: row.name, source: 'sync-stripe-prices.ts' },
        });
        productId = created.id;
        console.log(`  created Product ${productId} for "${row.name}"`);
      }
      productCache.set(row.name, productId);
    }

    // ── Create Price ──────────────────────────────────────────
    // Stripe Price unit_amount semantics:
    //   USD: cents (×100) — pass 499 for $4.99
    //   VND: whole VND (no multiplier) — pass 99000 for 99,000 VND
    // Our DB column already stores in Stripe-native units, so pass as-is.
    const interval = row.billing_period === 'yearly' ? 'year' : 'month';
    const price = await stripe.prices.create({
      product: productId,
      unit_amount: row.price_cents,
      currency: row.currency!.toLowerCase(),
      recurring: { interval },
      metadata: {
        plan_id: row.id,
        plan_name: row.name,
        billing_period: row.billing_period,
        source: 'sync-stripe-prices.ts',
      },
    });

    console.log(`  created Price ${price.id} (${row.price_cents} ${row.currency} / ${interval})`);

    // ── Update plan row ───────────────────────────────────────
    await ds.query(`UPDATE plans SET stripe_price_id = $1 WHERE id = $2`, [price.id, row.id]);
    console.log(`✓ ${label}: stamped ${price.id}\n`);
    updated++;
  }

  await ds.destroy();

  console.log('─'.repeat(60));
  console.log(`Done. ${updated} updated, ${skipped} skipped (already had price_id).`);
}

main().catch((err) => {
  console.error('FATAL:', err);
  process.exit(1);
});
