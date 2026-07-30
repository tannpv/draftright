/**
 * One-time migration for #50: encrypt existing plaintext secrets at rest.
 *
 * Phase 2 covers `ai_providers.api_key`. Idempotent — rows already in `enc:v1:`
 * form are skipped, so re-running is safe. Requires SECRETS_ENCRYPTION_KEY and
 * DATABASE_URL. Run AFTER the read-both code is deployed and the key is set:
 *
 *   SECRETS_ENCRYPTION_KEY=<base64-32b> DATABASE_URL=... \
 *     npx ts-node scripts/encrypt-secrets.ts
 */
import { Client } from 'pg';
import { encryptSecret, isEncrypted } from '../src/common/crypto/secret-cipher';

async function main(): Promise<void> {
  if (!process.env.SECRETS_ENCRYPTION_KEY) {
    console.error('SECRETS_ENCRYPTION_KEY must be set (base64 of 32 bytes).');
    process.exit(1);
  }
  const connectionString = process.env.DATABASE_URL;
  if (!connectionString) {
    console.error('DATABASE_URL is required.');
    process.exit(1);
  }

  const client = new Client({ connectionString });
  await client.connect();
  try {
    const { rows } = await client.query<{ id: string; api_key: string }>(
      `SELECT id, api_key FROM ai_providers WHERE api_key IS NOT NULL AND api_key <> ''`,
    );
    let encrypted = 0;
    let skipped = 0;
    for (const row of rows) {
      if (isEncrypted(row.api_key)) {
        skipped++;
        continue;
      }
      await client.query('UPDATE ai_providers SET api_key = $1 WHERE id = $2', [
        encryptSecret(row.api_key),
        row.id,
      ]);
      encrypted++;
    }
    console.log(
      `ai_providers.api_key: encrypted ${encrypted}, skipped ${skipped} (already encrypted).`,
    );
  } finally {
    await client.end();
  }
}

main().catch((err) => {
  console.error(err);
  process.exit(1);
});
