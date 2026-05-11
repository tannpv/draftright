/**
 * sync-stripe-prices.js — populate plans.stripe_price_id for paid rows.
 *
 * Run from inside the backend container:
 *   sudo docker exec -e STRIPE_SECRET_KEY=$KEY -e DATABASE_URL=$URL \
 *     draftright-backend-1 node /app/scripts/sync-stripe-prices.js
 *
 * Idempotent. Skips rows that already have stripe_price_id.
 */

const Stripe = require('stripe');
const { Client } = require('pg');

async function main() {
  const stripeKey = process.env.STRIPE_SECRET_KEY;
  if (!stripeKey) { console.error('STRIPE_SECRET_KEY required'); process.exit(1); }

  const databaseUrl = process.env.DATABASE_URL;
  if (!databaseUrl) { console.error('DATABASE_URL required'); process.exit(1); }

  const stripe = new Stripe(stripeKey);
  const db = new Client({ connectionString: databaseUrl });
  await db.connect();

  console.log('Connected. Loading paid plans...\n');

  const { rows } = await db.query(`
    SELECT id, name, daily_limit, price_cents, currency, stripe_price_id, trial_days, billing_period
    FROM plans
    WHERE price_cents > 0 AND is_active = true
    ORDER BY name, currency, billing_period
  `);
  console.log(`Found ${rows.length} paid plan rows.\n`);

  const productCache = new Map();
  let updated = 0, skipped = 0;

  for (const row of rows) {
    const label = `${row.name} ${row.currency} ${row.billing_period}`;

    if (row.stripe_price_id) {
      console.log(`✓ ${label}: already has ${row.stripe_price_id}`);
      skipped++;
      continue;
    }

    let productId = productCache.get(row.name);
    if (!productId) {
      const existing = await stripe.products.list({ active: true, limit: 100 });
      const found = existing.data.find((p) => p.name === `DraftRight ${row.name}`);
      if (found) {
        productId = found.id;
        console.log(`  reusing Product ${productId} for "${row.name}"`);
      } else {
        const created = await stripe.products.create({
          name: `DraftRight ${row.name}`,
          description: `${row.daily_limit === -1 ? 'Unlimited' : row.daily_limit} rewrites/day`,
          metadata: { plan_name: row.name, source: 'sync-stripe-prices.js' },
        });
        productId = created.id;
        console.log(`  created Product ${productId} for "${row.name}"`);
      }
      productCache.set(row.name, productId);
    }

    const interval = row.billing_period === 'yearly' ? 'year' : 'month';
    const price = await stripe.prices.create({
      product: productId,
      unit_amount: row.price_cents,
      currency: row.currency.toLowerCase(),
      recurring: { interval },
      metadata: {
        plan_id: row.id,
        plan_name: row.name,
        billing_period: row.billing_period,
        source: 'sync-stripe-prices.js',
      },
    });
    console.log(`  created Price ${price.id} (${row.price_cents} ${row.currency}/${interval})`);

    await db.query(`UPDATE plans SET stripe_price_id = $1 WHERE id = $2`, [price.id, row.id]);
    console.log(`✓ ${label}: stamped ${price.id}\n`);
    updated++;
  }

  await db.end();
  console.log('─'.repeat(60));
  console.log(`Done. ${updated} updated, ${skipped} skipped.`);
}

main().catch((err) => { console.error('FATAL:', err); process.exit(1); });
