import { test, expect } from '@playwright/test';

const ADMIN_EMAIL = 'admin@draftright.info';
const ADMIN_PASSWORD = 'MyP@ssword1';

// Marker so the created bug-report row can be found + deleted afterward.
const MARKER = `E2E-LARGE-IMG-${Date.now()}`;

test('Report-a-Bug accepts a >5MB image (client compresses it) and submits', async ({ page }) => {
  // --- Sign in ---
  await page.goto('/login');
  await page.locator('input[type="email"]').fill(ADMIN_EMAIL);
  await page.locator('input[type="password"]').fill(ADMIN_PASSWORD);
  await page.getByRole('button', { name: 'Sign In' }).click();
  // Wait for the authenticated shell (the Report-bug button lives in the layout).
  await expect(page.getByRole('button', { name: /Report bug/i })).toBeVisible();

  // --- Open the Report-a-Bug modal ---
  await page.getByRole('button', { name: /Report bug/i }).click();
  await page.getByPlaceholder('I clicked X and Y broke...').fill(`${MARKER} — automated test of large-screenshot compression`);

  // --- Inject a REAL >5MB PNG into the file input (random noise so PNG can't
  // compress it small), exactly as a paste/drop/pick would. This forces the
  // client-side downscaleImage() path. ---
  const rawBytes = await page.evaluate(async () => {
    const side = 3000; // 3000x3000 random noise → multi-MB PNG
    const c = document.createElement('canvas');
    c.width = side; c.height = side;
    const ctx = c.getContext('2d')!;
    const imgData = ctx.createImageData(side, side);
    for (let i = 0; i < imgData.data.length; i += 4) {
      imgData.data[i] = Math.random() * 256;
      imgData.data[i + 1] = Math.random() * 256;
      imgData.data[i + 2] = Math.random() * 256;
      imgData.data[i + 3] = 255;
    }
    ctx.putImageData(imgData, 0, 0);
    const blob: Blob = await new Promise((r) => c.toBlob((b) => r(b!), 'image/png'));
    const file = new File([blob], 'huge-screenshot.png', { type: 'image/png' });
    const input = document.querySelector('input[type="file"]') as HTMLInputElement;
    const dt = new DataTransfer();
    dt.items.add(file);
    input.files = dt.files;
    input.dispatchEvent(new Event('change', { bubbles: true }));
    return blob.size;
  });

  // The raw image must genuinely exceed the 5 MB limit, or the test proves nothing.
  expect(rawBytes).toBeGreaterThan(5 * 1024 * 1024);

  // No "must be under 5 MB" rejection should appear (compression handled it).
  await expect(page.getByText(/under 5 MB|too large/i)).toHaveCount(0);

  // --- Submit + assert success ---
  page.on('console', (m) => console.log('BROWSER:', m.type(), m.text()));
  page.on('requestfailed', (r) => console.log('REQFAIL:', r.url(), r.failure()?.errorText));
  const respPromise = page
    .waitForResponse((r) => r.url().includes('/bug-reports') && r.request().method() === 'POST', { timeout: 30_000 })
    .then(async (r) => console.log('POST /bug-reports ->', r.status(), (await r.text()).slice(0, 200)))
    .catch((e) => console.log('NO /bug-reports POST seen:', e.message));
  await page.getByRole('button', { name: 'Submit report' }).click();
  await respPromise;
  await expect(page.getByText("Thanks! We'll look into it.")).toBeVisible();
});
