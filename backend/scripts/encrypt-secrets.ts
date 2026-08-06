/**
 * One-time migration for #50: encrypt existing plaintext secrets at rest.
 *
 * Covers `ai_providers.api_key` (phase 2) and the app_settings secret columns
 * (phase 3). Idempotent — rows already in `enc:v1:` form are skipped, so
 * re-running is safe. Requires SECRETS_ENCRYPTION_KEY and DATABASE_URL. Run
 * AFTER the read-both code is deployed and the key is set:
 *
 *   SECRETS_ENCRYPTION_KEY=<base64-32b> DATABASE_URL=... \
 *     npx ts-node scripts/encrypt-secrets.ts
 */
import { Client } from 'pg';
import { encryptSecret, isEncrypted } from '../src/common/crypto/secret-cipher';
// Single source of truth for which app_settings columns are encrypted at rest.
import { SETTINGS_ENCRYPTED_COLUMNS as SECRET_COLUMNS } from '../src/admin/mask-settings.util';

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

    // app_settings singleton — encrypt the at-rest secret columns (imported).
    const settings = await client.query(
      `SELECT id, ${SECRET_COLUMNS.join(', ')} FROM app_settings`,
    );
    for (const row of settings.rows) {
      const updates: string[] = [];
      const values: string[] = [];
      for (const col of SECRET_COLUMNS) {
        const val = row[col];
        if (typeof val === 'string' && val !== '' && !isEncrypted(val)) {
          values.push(encryptSecret(val));
          updates.push(`${col} = $${values.length}`);
        }
      }
      if (updates.length === 0) continue;
      values.push(row.id);
      await client.query(
        `UPDATE app_settings SET ${updates.join(', ')} WHERE id = $${values.length}`,
        values,
      );
      console.log(`app_settings ${row.id}: encrypted ${updates.length} secret column(s).`);
    }
  } finally {
    await client.end();
  }
}

main().catch((err) => {
  console.error(err);
  process.exit(1);
});
