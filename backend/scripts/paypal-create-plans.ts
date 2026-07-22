/**
 * One-shot: create the PayPal Product + monthly/yearly Billing Plans that the
 * PayPalStrategy subscribes users to. Run once per environment (sandbox, then
 * live); paste the printed plan IDs into Admin → Settings → Payment
 * (paypal_plan_monthly / paypal_plan_yearly).
 *
 * Prices + trial come from the live /plans endpoint (single source of truth) —
 * nothing about pricing is hardcoded here. PayPal supports USD only, so only
 * USD Pro plans are used; VN buyers keep VietQR.
 *
 * Usage:
 *   PAYPAL_CLIENT_ID=... PAYPAL_CLIENT_SECRET=... PAYPAL_MODE=sandbox \
 *   BACKEND_URL=https://api.draftright.info \
 *   npx ts-node scripts/paypal-create-plans.ts
 */

const MODE = (process.env.PAYPAL_MODE || 'sandbox').toLowerCase();
const BASE = MODE === 'live' ? 'https://api-m.paypal.com' : 'https://api-m.sandbox.paypal.com';
const BACKEND_URL = process.env.BACKEND_URL || 'http://localhost:3000';
const CLIENT_ID = process.env.PAYPAL_CLIENT_ID || '';
const CLIENT_SECRET = process.env.PAYPAL_CLIENT_SECRET || '';

async function token(): Promise<string> {
  if (!CLIENT_ID || !CLIENT_SECRET) throw new Error('Set PAYPAL_CLIENT_ID + PAYPAL_CLIENT_SECRET.');
  const basic = Buffer.from(`${CLIENT_ID}:${CLIENT_SECRET}`).toString('base64');
  const res = await fetch(`${BASE}/v1/oauth2/token`, {
    method: 'POST',
    headers: { Authorization: `Basic ${basic}`, 'Content-Type': 'application/x-www-form-urlencoded' },
    body: 'grant_type=client_credentials',
  });
  if (!res.ok) throw new Error(`PayPal auth failed ${res.status}: ${await res.text()}`);
  return (await res.json()).access_token;
}

type PlanRow = { name: string; price_cents: number; currency: string; billing_period: string; trial_days: number; is_active: boolean };

async function loadUsdProPlans(): Promise<{ monthly: PlanRow; yearly: PlanRow }> {
  const res = await fetch(`${BACKEND_URL}/plans`);
  if (!res.ok) throw new Error(`GET ${BACKEND_URL}/plans failed ${res.status}`);
  const rows: PlanRow[] = await res.json();
  const usd = rows.filter((p) => p.is_active && p.currency === 'USD' && p.price_cents > 0);
  const monthly = usd.find((p) => p.billing_period === 'monthly');
  const yearly = usd.find((p) => p.billing_period === 'yearly');
  if (!monthly || !yearly) throw new Error('Could not find active USD monthly + yearly plans in /plans.');
  return { monthly, yearly };
}

async function createProduct(t: string): Promise<string> {
  const res = await fetch(`${BASE}/v1/catalogs/products`, {
    method: 'POST',
    headers: { Authorization: `Bearer ${t}`, 'Content-Type': 'application/json' },
    body: JSON.stringify({ name: 'DraftRight Pro', type: 'SERVICE', category: 'SOFTWARE' }),
  });
  if (!res.ok) throw new Error(`Create product failed ${res.status}: ${await res.text()}`);
  return (await res.json()).id;
}

async function createPlan(t: string, productId: string, plan: PlanRow): Promise<string> {
  const isYearly = plan.billing_period === 'yearly';
  const value = (plan.price_cents / 100).toFixed(2);
  const billing_cycles: any[] = [];
  let sequence = 1;
  if (plan.trial_days > 0) {
    billing_cycles.push({
      frequency: { interval_unit: 'DAY', interval_count: plan.trial_days },
      tenure_type: 'TRIAL', sequence: sequence++, total_cycles: 1,
      pricing_scheme: { fixed_price: { value: '0', currency_code: plan.currency } },
    });
  }
  billing_cycles.push({
    frequency: { interval_unit: isYearly ? 'YEAR' : 'MONTH', interval_count: 1 },
    tenure_type: 'REGULAR', sequence: sequence, total_cycles: 0,
    pricing_scheme: { fixed_price: { value, currency_code: plan.currency } },
  });
  const res = await fetch(`${BASE}/v1/billing/plans`, {
    method: 'POST',
    headers: { Authorization: `Bearer ${t}`, 'Content-Type': 'application/json' },
    body: JSON.stringify({
      product_id: productId,
      name: `DraftRight Pro — ${isYearly ? 'Yearly' : 'Monthly'}`,
      billing_cycles,
      payment_preferences: { auto_bill_outstanding: true, setup_fee_failure_action: 'CONTINUE', payment_failure_threshold: 3 },
    }),
  });
  if (!res.ok) throw new Error(`Create plan failed ${res.status}: ${await res.text()}`);
  return (await res.json()).id;
}

async function main() {
  console.log(`PayPal plan bootstrap — mode=${MODE}, base=${BASE}, plans from ${BACKEND_URL}/plans`);
  const { monthly, yearly } = await loadUsdProPlans();
  console.log(`Prices: monthly $${(monthly.price_cents / 100).toFixed(2)}, yearly $${(yearly.price_cents / 100).toFixed(2)}, trial ${monthly.trial_days}d`);
  const t = await token();
  const productId = await createProduct(t);
  console.log(`Product: ${productId}`);
  const monthlyId = await createPlan(t, productId, monthly);
  const yearlyId = await createPlan(t, productId, yearly);
  console.log('\n=== Paste into Admin → Settings → Payment ===');
  console.log(`paypal_plan_monthly = ${monthlyId}`);
  console.log(`paypal_plan_yearly  = ${yearlyId}`);
}

main().catch((e) => { console.error(e.message || e); process.exit(1); });
