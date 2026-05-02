# Customer Registration Flow Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Public customers can register on the website, verify their email, and start using DraftRight on any platform — replacing the current admin-only manual provisioning.

**Architecture:** Email + password registration with 6-digit OTP email verification (Resend). Soft-gate (rewrites work while unverified, banner nags). All client apps deep-link to a single web signup page. Existing social-login path (`/auth/social`) is unchanged and serves as the second track.

**Tech Stack:** NestJS (backend), TypeORM + PostgreSQL, Resend (email), Astro + React island (signup pages), existing JWT flow.

---

## File Structure

**Backend (`backend/src/`):**
- Modify: `users/entities/user.entity.ts` — add `email_verified`, `email_verification_code`, `email_verification_expires` columns
- Create: `email/email.module.ts`, `email/email.service.ts`, `email/templates/verification.ts` — Resend wrapper + template
- Modify: `auth/auth.module.ts` — import EmailModule
- Modify: `auth/auth.service.ts` — generate + persist code on register, add `verifyEmail`, `resendVerification`
- Modify: `auth/auth.controller.ts` — add `POST /auth/verify-email`, `POST /auth/resend-verification`
- Modify: `app.module.ts` — register EmailModule globally
- Create: `migrations/2026-04-30-email-verification.sql` — column additions for prod (synchronize is OFF)

**Website (`website/src/`):**
- Create: `pages/signup.astro` — wraps the SignupForm island
- Create: `pages/verify-email.astro` — wraps the VerifyEmailForm island
- Create: `components/SignupForm.tsx` — React form: name/email/password → POST /auth/register → redirect to /verify-email
- Create: `components/VerifyEmailForm.tsx` — 6-digit input → POST /auth/verify-email → success
- Modify: `components/Nav.astro` — add "Sign Up" CTA

**Clients (deep-link only — no registration UI duplicated):**
- Modify: `DraftRight/UI/Settings/AccountSettingsTab.swift` — remove pre-filled creds, add "Create account" link opening `https://draftright.info/signup`
- Modify: `DraftRightMobile/lib/screens/login_screen.dart` — same: remove pre-fill + add link

**Pre-launch hygiene:**
- Modify: backend seed / direct SQL — drop free plan `daily_limit` from `-1` to `20`

---

## Self-Review Checklist (run after writing)

- [ ] All registration paths covered: web, deep-link from clients, social fallback
- [ ] Email verification is **soft-gate** (banner) per design — not hard-gate
- [ ] No backwards-compat code for "users without verification flag" — entity has default `false`, prod migration sets existing users to `true` (grandfathered)
- [ ] Pre-filled credentials removed before any commit hits main
- [ ] Schema change deployment path documented (NODE_ENV=staging dance OR raw SQL migration)

---

## Stage 1: Backend — Email Verification Schema

### Task 1: Add email verification columns to User entity

**Files:**
- Modify: `backend/src/users/entities/user.entity.ts`
- Test: `backend/src/users/users.service.spec.ts`

- [ ] **Step 1: Write the failing test**

```typescript
// In users.service.spec.ts — new describe block
describe('email verification fields', () => {
  it('creates a user with email_verified=false by default', async () => {
    const user = await service.create({
      email: 'verify-test@example.com',
      password_hash: 'x',
      name: 'V',
    });
    expect(user.email_verified).toBe(false);
    expect(user.email_verification_code).toBeNull();
  });
});
```

- [ ] **Step 2: Run the test to confirm failure**

Run: `cd backend && npm test -- users.service`
Expected: FAIL — `email_verified` not on entity.

- [ ] **Step 3: Add columns to entity**

In `user.entity.ts`, add after the existing `is_active` column:

```typescript
@Column({ type: 'boolean', default: false })
email_verified: boolean;

@Column({ type: 'varchar', length: 6, nullable: true })
email_verification_code: string | null;

@Column({ type: 'timestamptz', nullable: true })
email_verification_expires: Date | null;
```

- [ ] **Step 4: Run the test — should pass**

Run: `cd backend && npm test -- users.service`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add backend/src/users/entities/user.entity.ts backend/src/users/users.service.spec.ts
git commit -m "feat(backend): add email verification columns to User entity"
```

---

### Task 2: Production migration for existing users

**Files:**
- Create: `backend/migrations/2026-04-30-email-verification.sql`

- [ ] **Step 1: Write migration**

Create `backend/migrations/2026-04-30-email-verification.sql`:

```sql
ALTER TABLE users ADD COLUMN IF NOT EXISTS email_verified boolean NOT NULL DEFAULT false;
ALTER TABLE users ADD COLUMN IF NOT EXISTS email_verification_code varchar(6);
ALTER TABLE users ADD COLUMN IF NOT EXISTS email_verification_expires timestamptz;

-- Grandfather existing users (admin + test accounts created before verification existed)
UPDATE users SET email_verified = true WHERE email_verified = false;
```

- [ ] **Step 2: Verify against current prod schema**

Run: `ssh draftright "docker exec draftright-postgres-1 psql -U draftright -d draftright -c '\\d users'"`
Expected: current columns listed (no email_verified yet) — proves migration is needed.

- [ ] **Step 3: Apply migration to prod**

```bash
scp backend/migrations/2026-04-30-email-verification.sql draftright:/tmp/
ssh draftright "docker exec -i draftright-postgres-1 psql -U draftright -d draftright < /tmp/2026-04-30-email-verification.sql"
ssh draftright "docker exec draftright-postgres-1 psql -U draftright -d draftright -c '\\d users' | grep email_verified"
```

Expected: column listed with `boolean NOT NULL DEFAULT false`.

- [ ] **Step 4: Commit**

```bash
git add backend/migrations/2026-04-30-email-verification.sql
git commit -m "feat(backend): migration for email verification columns"
```

---

## Stage 2: Backend — Resend Email Service

### Task 3: Sign up for Resend and add API key

- [ ] **Step 1: Manual — sign up at https://resend.com**

Free tier: 3,000 emails/month, 100/day. Adequate for Phase 1 launch.

- [ ] **Step 2: Verify domain `draftright.info`**

Add the DNS records Resend provides (SPF, DKIM, MX) at GoDaddy. Wait for verification (~10 min).

- [ ] **Step 3: Generate API key**

Resend dashboard → API Keys → Create. Scope: **Sending access**, restricted to `draftright.info`.

- [ ] **Step 4: Add to prod .env**

```bash
ssh draftright "echo 'RESEND_API_KEY=re_xxxxx' >> /opt/draftright/.env"
ssh draftright "echo 'EMAIL_FROM=DraftRight <noreply@draftright.info>' >> /opt/draftright/.env"
```

Add to `backend/.env.example`:

```
RESEND_API_KEY=re_xxx_your_key_here
EMAIL_FROM=DraftRight <noreply@draftright.info>
```

- [ ] **Step 5: Add to docker-compose.prod.yml**

In `/opt/draftright/docker-compose.prod.yml` `backend.environment`, add:

```yaml
      - RESEND_API_KEY=${RESEND_API_KEY}
      - EMAIL_FROM=${EMAIL_FROM}
```

---

### Task 4: Write EmailService

**Files:**
- Create: `backend/src/email/email.module.ts`
- Create: `backend/src/email/email.service.ts`
- Create: `backend/src/email/email.service.spec.ts`

- [ ] **Step 1: Install Resend SDK**

```bash
cd backend && npm install resend
```

- [ ] **Step 2: Write the failing test**

Create `backend/src/email/email.service.spec.ts`:

```typescript
import { Test } from '@nestjs/testing';
import { EmailService } from './email.service';

describe('EmailService', () => {
  let service: EmailService;
  let sendMock: jest.Mock;

  beforeEach(async () => {
    sendMock = jest.fn().mockResolvedValue({ data: { id: 'email_123' }, error: null });
    process.env.RESEND_API_KEY = 'test-key';
    process.env.EMAIL_FROM = 'noreply@draftright.info';

    const module = await Test.createTestingModule({
      providers: [EmailService],
    }).compile();
    service = module.get(EmailService);
    (service as any).client = { emails: { send: sendMock } };
  });

  it('sends a verification email with a 6-digit code', async () => {
    await service.sendVerificationEmail('user@example.com', 'Tan', '123456');
    expect(sendMock).toHaveBeenCalledTimes(1);
    const call = sendMock.mock.calls[0][0];
    expect(call.to).toBe('user@example.com');
    expect(call.subject).toContain('Verify');
    expect(call.html).toContain('123456');
    expect(call.html).toContain('Tan');
  });

  it('throws if Resend returns an error', async () => {
    sendMock.mockResolvedValueOnce({ data: null, error: { message: 'rate limited' } });
    await expect(service.sendVerificationEmail('a@b.com', 'A', '111111'))
      .rejects.toThrow(/rate limited/);
  });
});
```

- [ ] **Step 3: Run — should fail (no service)**

Run: `cd backend && npm test -- email.service`
Expected: FAIL.

- [ ] **Step 4: Implement EmailService**

Create `backend/src/email/email.service.ts`:

```typescript
import { Injectable, Logger, InternalServerErrorException } from '@nestjs/common';
import { Resend } from 'resend';

@Injectable()
export class EmailService {
  private readonly logger = new Logger(EmailService.name);
  private client = new Resend(process.env.RESEND_API_KEY);
  private from = process.env.EMAIL_FROM ?? 'DraftRight <noreply@draftright.info>';

  async sendVerificationEmail(toEmail: string, name: string, code: string): Promise<void> {
    const html = this.renderVerification(name, code);
    const result = await this.client.emails.send({
      from: this.from,
      to: toEmail,
      subject: 'Verify your DraftRight email',
      html,
    });
    if (result.error) {
      this.logger.error(`Resend error: ${result.error.message}`);
      throw new InternalServerErrorException(`Email send failed: ${result.error.message}`);
    }
    this.logger.log(`Verification email sent to ${toEmail} (id=${result.data?.id})`);
  }

  private renderVerification(name: string, code: string): string {
    return `
<!doctype html>
<html><body style="font-family:-apple-system,system-ui,sans-serif;background:#f5f5f7;padding:32px;">
  <div style="max-width:480px;margin:0 auto;background:#fff;border-radius:12px;padding:32px;">
    <h1 style="font-size:20px;margin:0 0 16px;">Welcome to DraftRight, ${this.escapeHtml(name)}</h1>
    <p style="color:#444;line-height:1.5;">Confirm your email by entering this 6-digit code in the app or on the verification page:</p>
    <div style="font-size:32px;font-weight:600;letter-spacing:8px;text-align:center;background:#f0f0f0;padding:16px;border-radius:8px;margin:24px 0;">${code}</div>
    <p style="color:#888;font-size:13px;">This code expires in 15 minutes. If you didn't sign up, you can safely ignore this email.</p>
  </div>
</body></html>`;
  }

  private escapeHtml(s: string): string {
    return s.replace(/[&<>"']/g, c => ({ '&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;' }[c]!));
  }
}
```

- [ ] **Step 5: Create EmailModule**

Create `backend/src/email/email.module.ts`:

```typescript
import { Module, Global } from '@nestjs/common';
import { EmailService } from './email.service';

@Global()
@Module({
  providers: [EmailService],
  exports: [EmailService],
})
export class EmailModule {}
```

- [ ] **Step 6: Register globally**

Modify `backend/src/app.module.ts`: add `EmailModule` to `imports`.

- [ ] **Step 7: Run tests — should pass**

Run: `cd backend && npm test -- email.service`
Expected: 2/2 PASS.

- [ ] **Step 8: Commit**

```bash
git add backend/src/email backend/src/app.module.ts backend/package.json backend/package-lock.json
git commit -m "feat(backend): EmailService with Resend integration"
```

---

## Stage 3: Backend — Verification Endpoints

### Task 5: Generate code on registration + send email

**Files:**
- Modify: `backend/src/auth/auth.service.ts`
- Modify: `backend/src/auth/auth.module.ts` (already has EmailModule via global, but verify)
- Test: `backend/src/auth/auth.service.spec.ts`

- [ ] **Step 1: Write the failing test**

```typescript
// In auth.service.spec.ts
describe('register with email verification', () => {
  it('persists a 6-digit code and 15-min expiry', async () => {
    const result = await service.register('verify@example.com', 'pw', 'V');
    const fresh = await usersService.findById(result.user.id);
    expect(fresh!.email_verification_code).toMatch(/^\d{6}$/);
    expect(fresh!.email_verified).toBe(false);
    const expectedExpiry = Date.now() + 15 * 60 * 1000;
    expect(fresh!.email_verification_expires!.getTime()).toBeGreaterThan(expectedExpiry - 5000);
    expect(fresh!.email_verification_expires!.getTime()).toBeLessThan(expectedExpiry + 5000);
  });

  it('calls EmailService.sendVerificationEmail', async () => {
    const sendSpy = jest.spyOn(emailService, 'sendVerificationEmail').mockResolvedValue();
    await service.register('a@b.com', 'pw', 'A');
    expect(sendSpy).toHaveBeenCalledWith('a@b.com', 'A', expect.stringMatching(/^\d{6}$/));
  });
});
```

(Wire `emailService` mock in `beforeEach` similar to existing service mocks.)

- [ ] **Step 2: Run — should fail**

Run: `cd backend && npm test -- auth.service`
Expected: FAIL.

- [ ] **Step 3: Modify register to generate + persist code + send email**

In `auth.service.ts`, inject `EmailService` and update `register`:

```typescript
constructor(
  private readonly usersService: UsersService,
  private readonly jwtService: JwtService,
  private readonly plansService: PlansService,
  private readonly subscriptionsService: SubscriptionsService,
  private readonly emailService: EmailService,
) {}

async register(email: string, password: string, name: string) {
  const normalizedEmail = email.trim().toLowerCase();
  const existing = await this.usersService.findByEmail(normalizedEmail);
  if (existing) throw new ConflictException('Email already registered');

  const password_hash = await bcrypt.hash(password, 10);
  const code = this.generateCode();
  const expires = new Date(Date.now() + 15 * 60 * 1000);

  const user = await this.usersService.create({
    email: normalizedEmail,
    password_hash,
    name,
    email_verification_code: code,
    email_verification_expires: expires,
  } as any);

  const freePlan = await this.plansService.findFreePlan();
  await this.subscriptionsService.createFreeSubscription(user.id, freePlan.id);

  // Fire-and-forget email — don't block registration on Resend latency.
  this.emailService.sendVerificationEmail(normalizedEmail, name, code).catch(err => {
    Logger.error(`Failed to send verification email: ${err.message}`, 'AuthService');
  });

  const tokens = this.generateTokens(user);
  return { ...tokens, user: { id: user.id, email: user.email, name: user.name, email_verified: false } };
}

private generateCode(): string {
  return Math.floor(100000 + Math.random() * 900000).toString();
}
```

- [ ] **Step 4: Run tests — should pass**

Run: `cd backend && npm test -- auth.service`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add backend/src/auth/auth.service.ts backend/src/auth/auth.service.spec.ts
git commit -m "feat(backend): generate verification code on register and send email"
```

---

### Task 6: Verify and resend endpoints

**Files:**
- Modify: `backend/src/auth/auth.service.ts`
- Modify: `backend/src/auth/auth.controller.ts`
- Test: `backend/src/auth/auth.service.spec.ts`

- [ ] **Step 1: Write the failing tests**

```typescript
describe('verifyEmail', () => {
  it('marks email_verified=true and clears code on correct code', async () => {
    const reg = await service.register('v1@example.com', 'pw', 'V');
    const fresh = await usersService.findById(reg.user.id);
    await service.verifyEmail('v1@example.com', fresh!.email_verification_code!);
    const after = await usersService.findById(reg.user.id);
    expect(after!.email_verified).toBe(true);
    expect(after!.email_verification_code).toBeNull();
  });

  it('rejects wrong code', async () => {
    await service.register('v2@example.com', 'pw', 'V');
    await expect(service.verifyEmail('v2@example.com', '000000'))
      .rejects.toThrow('Invalid or expired verification code');
  });

  it('rejects expired code', async () => {
    const reg = await service.register('v3@example.com', 'pw', 'V');
    await usersService.update(reg.user.id, {
      email_verification_expires: new Date(Date.now() - 1000),
    } as any);
    const fresh = await usersService.findById(reg.user.id);
    await expect(service.verifyEmail('v3@example.com', fresh!.email_verification_code!))
      .rejects.toThrow('Invalid or expired verification code');
  });
});

describe('resendVerification', () => {
  it('issues a new code and sends an email', async () => {
    const reg = await service.register('r1@example.com', 'pw', 'R');
    const before = (await usersService.findById(reg.user.id))!.email_verification_code;
    const sendSpy = jest.spyOn(emailService, 'sendVerificationEmail').mockResolvedValue();
    await service.resendVerification('r1@example.com');
    const after = (await usersService.findById(reg.user.id))!.email_verification_code;
    expect(after).not.toBe(before);
    expect(sendSpy).toHaveBeenCalled();
  });

  it('is a no-op for unknown email (no enumeration)', async () => {
    await expect(service.resendVerification('nobody@example.com')).resolves.toBeUndefined();
  });
});
```

- [ ] **Step 2: Run — should fail**

Expected: 5 failures.

- [ ] **Step 3: Implement methods**

In `auth.service.ts`:

```typescript
async verifyEmail(email: string, code: string): Promise<{ success: true }> {
  const user = await this.usersService.findByEmail(email.trim().toLowerCase());
  if (!user || !user.email_verification_code || !user.email_verification_expires) {
    throw new BadRequestException('Invalid or expired verification code');
  }
  if (user.email_verification_code !== code) {
    throw new BadRequestException('Invalid or expired verification code');
  }
  if (user.email_verification_expires.getTime() < Date.now()) {
    throw new BadRequestException('Invalid or expired verification code');
  }
  await this.usersService.update(user.id, {
    email_verified: true,
    email_verification_code: null,
    email_verification_expires: null,
  } as any);
  return { success: true };
}

async resendVerification(email: string): Promise<void> {
  const user = await this.usersService.findByEmail(email.trim().toLowerCase());
  if (!user || user.email_verified) return; // Silent — don't leak existence
  const code = this.generateCode();
  const expires = new Date(Date.now() + 15 * 60 * 1000);
  await this.usersService.update(user.id, {
    email_verification_code: code,
    email_verification_expires: expires,
  } as any);
  this.emailService.sendVerificationEmail(user.email, user.name, code).catch(err => {
    Logger.error(`Failed to resend: ${err.message}`, 'AuthService');
  });
}
```

- [ ] **Step 4: Add controller endpoints**

In `auth.controller.ts`:

```typescript
@Post('verify-email')
@HttpCode(HttpStatus.OK)
async verifyEmail(@Body() body: { email: string; code: string }) {
  return this.authService.verifyEmail(body.email, body.code);
}

@Post('resend-verification')
@HttpCode(HttpStatus.OK)
async resendVerification(@Body() body: { email: string }) {
  await this.authService.resendVerification(body.email);
  return { success: true };
}
```

- [ ] **Step 5: Run tests — should pass**

Run: `cd backend && npm test -- auth`
Expected: all passing.

- [ ] **Step 6: Commit**

```bash
git add backend/src/auth
git commit -m "feat(backend): /auth/verify-email and /auth/resend-verification endpoints"
```

---

### Task 7: Deploy backend to prod

- [ ] **Step 1: Run schema migration first** (Task 2 step 3 — if not done yet)

- [ ] **Step 2: Sync changed files**

```bash
rsync -av --exclude=node_modules --exclude=dist backend/src/ draftright:/opt/draftright/src/
rsync -av backend/package.json backend/package-lock.json draftright:/opt/draftright/
```

- [ ] **Step 3: Rebuild backend container**

```bash
ssh draftright "cd /opt/draftright && docker compose -f docker-compose.prod.yml up -d --build backend"
```

- [ ] **Step 4: Smoke-test on prod**

```bash
# Register a fresh test account
curl -sS -X POST https://api.draftright.info/auth/register \
  -H "Content-Type: application/json" \
  -d '{"email":"smoke-test-'$(date +%s)'@draftright.info","password":"TestPw123!","name":"Smoke"}' | python3 -m json.tool

# Check the email arrives at the configured inbox
# Then verify with the code
curl -sS -X POST https://api.draftright.info/auth/verify-email \
  -H "Content-Type: application/json" \
  -d '{"email":"<the email>","code":"<the code from inbox>"}'
# Expected: {"success":true}
```

---

## Stage 4: Website — Signup + Verify Pages

### Task 8: Build SignupForm component

**Files:**
- Create: `website/src/components/SignupForm.tsx`
- Create: `website/src/pages/signup.astro`

- [ ] **Step 1: Create SignupForm**

```tsx
// website/src/components/SignupForm.tsx
import { useState } from 'react';

const API = import.meta.env.PUBLIC_API_URL || 'https://api.draftright.info';

export default function SignupForm() {
  const [name, setName] = useState('');
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const onSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError(null);
    setSubmitting(true);
    try {
      const res = await fetch(`${API}/auth/register`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ name, email: email.trim().toLowerCase(), password }),
      });
      if (!res.ok) {
        const body = await res.json().catch(() => ({}));
        throw new Error(body.message || `HTTP ${res.status}`);
      }
      window.location.href = `/verify-email?email=${encodeURIComponent(email)}`;
    } catch (err: any) {
      setError(err.message);
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <form onSubmit={onSubmit} className="max-w-md mx-auto space-y-4">
      <input className="w-full border rounded-lg p-3" placeholder="Your name"
             value={name} onChange={e => setName(e.target.value)} required minLength={2} />
      <input className="w-full border rounded-lg p-3" type="email" placeholder="Email"
             value={email} onChange={e => setEmail(e.target.value)} required />
      <input className="w-full border rounded-lg p-3" type="password" placeholder="Password (min 8 chars)"
             value={password} onChange={e => setPassword(e.target.value)} required minLength={8} />
      {error && <p className="text-red-600 text-sm">{error}</p>}
      <button className="w-full bg-black text-white rounded-lg p-3 font-medium disabled:opacity-50"
              type="submit" disabled={submitting}>
        {submitting ? 'Creating…' : 'Create account'}
      </button>
      <p className="text-sm text-center text-gray-600">
        Already have an account? <a href="/login" className="underline">Sign in</a>
      </p>
    </form>
  );
}
```

- [ ] **Step 2: Create the page wrapper**

```astro
---
// website/src/pages/signup.astro
import Layout from '../layouts/Layout.astro';
import SignupForm from '../components/SignupForm';
---
<Layout title="Sign up — DraftRight">
  <main class="py-16 px-6">
    <h1 class="text-3xl font-bold text-center mb-8">Create your DraftRight account</h1>
    <SignupForm client:load />
  </main>
</Layout>
```

(Verify `Layout.astro` and the existing layout pattern match — reference `index.astro`.)

- [ ] **Step 3: Smoke test locally**

```bash
cd website && npm run dev
# Open http://localhost:4000/signup, fill form against staging or prod
# Verify redirect to /verify-email?email=...
```

- [ ] **Step 4: Commit**

```bash
git add website/src/components/SignupForm.tsx website/src/pages/signup.astro
git commit -m "feat(website): /signup page with email + password registration"
```

---

### Task 9: Build VerifyEmailForm component

**Files:**
- Create: `website/src/components/VerifyEmailForm.tsx`
- Create: `website/src/pages/verify-email.astro`

- [ ] **Step 1: Create VerifyEmailForm**

```tsx
// website/src/components/VerifyEmailForm.tsx
import { useState, useEffect } from 'react';

const API = import.meta.env.PUBLIC_API_URL || 'https://api.draftright.info';

export default function VerifyEmailForm() {
  const [email, setEmail] = useState('');
  const [code, setCode] = useState('');
  const [submitting, setSubmitting] = useState(false);
  const [resending, setResending] = useState(false);
  const [done, setDone] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    const params = new URLSearchParams(window.location.search);
    const e = params.get('email');
    if (e) setEmail(e);
  }, []);

  const verify = async (e: React.FormEvent) => {
    e.preventDefault();
    setError(null);
    setSubmitting(true);
    try {
      const res = await fetch(`${API}/auth/verify-email`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ email, code }),
      });
      if (!res.ok) {
        const body = await res.json().catch(() => ({}));
        throw new Error(body.message || `HTTP ${res.status}`);
      }
      setDone(true);
    } catch (err: any) {
      setError(err.message);
    } finally {
      setSubmitting(false);
    }
  };

  const resend = async () => {
    setResending(true);
    setError(null);
    try {
      await fetch(`${API}/auth/resend-verification`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ email }),
      });
    } finally {
      setResending(false);
    }
  };

  if (done) {
    return (
      <div className="max-w-md mx-auto text-center space-y-4">
        <p className="text-green-600 text-lg">Email verified — you're all set.</p>
        <a href="/download" className="inline-block bg-black text-white rounded-lg px-6 py-3">Download the app</a>
      </div>
    );
  }

  return (
    <form onSubmit={verify} className="max-w-md mx-auto space-y-4">
      <p className="text-center text-gray-600">We sent a 6-digit code to <strong>{email || 'your email'}</strong></p>
      <input className="w-full border rounded-lg p-3 text-center text-2xl tracking-widest font-mono"
             value={code} onChange={e => setCode(e.target.value.replace(/\D/g, '').slice(0, 6))}
             placeholder="••••••" maxLength={6} required />
      {error && <p className="text-red-600 text-sm">{error}</p>}
      <button className="w-full bg-black text-white rounded-lg p-3 font-medium disabled:opacity-50"
              type="submit" disabled={submitting || code.length !== 6}>
        {submitting ? 'Verifying…' : 'Verify email'}
      </button>
      <button type="button" onClick={resend} disabled={resending}
              className="w-full text-sm text-gray-600 underline">
        {resending ? 'Sending…' : "Didn't get the code? Resend"}
      </button>
    </form>
  );
}
```

- [ ] **Step 2: Create the page wrapper**

```astro
---
// website/src/pages/verify-email.astro
import Layout from '../layouts/Layout.astro';
import VerifyEmailForm from '../components/VerifyEmailForm';
---
<Layout title="Verify your email — DraftRight">
  <main class="py-16 px-6">
    <h1 class="text-3xl font-bold text-center mb-8">Verify your email</h1>
    <VerifyEmailForm client:load />
  </main>
</Layout>
```

- [ ] **Step 3: Test full registration flow locally**

1. Open `/signup`, register fresh account
2. Get redirected to `/verify-email?email=...`
3. Check email inbox for code
4. Enter code, submit, see success state

- [ ] **Step 4: Commit**

```bash
git add website/src/components/VerifyEmailForm.tsx website/src/pages/verify-email.astro
git commit -m "feat(website): /verify-email page with 6-digit code input"
```

---

### Task 10: Add Sign Up CTA to Nav

**Files:**
- Modify: `website/src/components/Nav.astro`

- [ ] **Step 1: Read existing Nav**

Run: `cat website/src/components/Nav.astro`

- [ ] **Step 2: Add the CTA**

In the right-side button area (near Download / Pricing links), add:

```astro
<a href="/signup" class="bg-black text-white rounded-lg px-4 py-2 hover:bg-gray-800">Sign up</a>
```

(Match the spacing/style pattern of existing links.)

- [ ] **Step 3: Build website + visual check**

```bash
cd website && npm run build && npm run preview
# Verify nav renders the Sign Up CTA on / and /pricing
```

- [ ] **Step 4: Commit**

```bash
git add website/src/components/Nav.astro
git commit -m "feat(website): Sign Up CTA in nav"
```

---

### Task 11: Deploy website to prod

- [ ] **Step 1: Build production bundle**

```bash
cd website && npm run build
```

- [ ] **Step 2: Sync to prod**

```bash
rsync -av --delete website/dist/ draftright:/var/www/draftright/
```

- [ ] **Step 3: Smoke test live**

Open `https://draftright.info/signup` — register a real test account. Verify the email arrives at a real inbox. Enter the code at `/verify-email`. See success state.

---

## Stage 5: Clients — Deep-Link to Signup

### Task 12: macOS — remove pre-filled creds + add Sign Up link

**Files:**
- Modify: `DraftRight/UI/Settings/AccountSettingsTab.swift`

- [ ] **Step 1: Remove pre-filled credentials**

```swift
// Replace lines 6-8:
@State private var loginEmail: String = ""
@State private var loginPassword: String = ""
```

- [ ] **Step 2: Add a Create-account button below the Sign In row**

After the existing `HStack { Spacer(); Button("Sign In") {...} }` block, before the `Divider()`:

```swift
HStack {
    Text("Don't have an account?")
        .font(.caption)
        .foregroundColor(.secondary)
    Button("Create one") {
        if let url = URL(string: "https://draftright.info/signup") {
            NSWorkspace.shared.open(url)
        }
    }
    .buttonStyle(.link)
    .font(.caption)
    Spacer()
}
```

- [ ] **Step 3: Build + manual test**

```bash
cd /opt/openAi/DraftRight && swift build -c release
codesign --force --deep --sign - .build/release/DraftRight.app  # if SPM produces .app
# Or distribute the existing /Applications/DraftRight.app after copying the rebuilt binary
```

Open Settings → Account: pre-filled fields gone, "Create one" link opens browser to `/signup`.

- [ ] **Step 4: Commit**

```bash
git add DraftRight/UI/Settings/AccountSettingsTab.swift
git commit -m "feat(macos): remove pre-filled creds, add Create-account link to signup"
```

---

### Task 13: Flutter mobile — same change

**Files:**
- Modify: `DraftRightMobile/lib/screens/login_screen.dart`

- [ ] **Step 1: Remove pre-filled credentials**

Find the `TextEditingController` initial values and clear them.

- [ ] **Step 2: Add Sign Up link**

Below the Sign In button, add:

```dart
TextButton(
  onPressed: () => launchUrl(Uri.parse('https://draftright.info/signup')),
  child: const Text("Don't have an account? Sign up"),
),
```

(Verify `url_launcher` is in pubspec — it likely already is given existing OAuth flows.)

- [ ] **Step 3: Build + manual test**

```bash
cd DraftRightMobile && flutter build apk --release
# Sideload to a connected Android, open login screen, tap link, verify browser opens /signup
```

- [ ] **Step 4: Commit (in submodule)**

```bash
cd DraftRightMobile
git add lib/screens/login_screen.dart
git commit -m "feat(mobile): remove pre-filled creds, add signup link"
cd ..
git add DraftRightMobile
git commit -m "chore: bump mobile submodule with signup link"
```

---

## Stage 6: Pre-Launch Hygiene

### Task 14: Drop free plan daily_limit to 20

- [ ] **Step 1: Update via SQL on prod**

```bash
ssh draftright "docker exec -i draftright-postgres-1 psql -U draftright -d draftright -c \"UPDATE plans SET daily_limit = 20 WHERE name = 'Free';\""
ssh draftright "docker exec draftright-postgres-1 psql -U draftright -d draftright -c \"SELECT name, daily_limit FROM plans;\""
```

Expected: Free plan shows `daily_limit | 20`.

- [ ] **Step 2: Update seed file for fresh deploys**

Find and modify `backend/src/seed.ts` — change Free plan creation from `daily_limit: -1` to `daily_limit: 20`.

- [ ] **Step 3: Smoke-test the limit**

Use the test account from Task 7 to issue 21 rewrites. The 21st should be rejected with a quota error.

- [ ] **Step 4: Commit**

```bash
git add backend/src/seed.ts
git commit -m "feat(backend): Free plan daily limit 20 (was unlimited dogfooding)"
```

---

### Task 15: Final pre-launch verification

- [ ] **Step 1: Manual end-to-end test**

1. Browser incognito → `https://draftright.info/signup`
2. Register a new account with a real email you own
3. Receive verification code at the inbox
4. Enter code at `/verify-email` → see success
5. Download macOS app from `/download`
6. Open Settings → Account → Sign in with the new account → verify works
7. Issue a rewrite — succeeds
8. Issue 20 more — 21st rejected (quota)

- [ ] **Step 2: Update memory**

Save a note in `~/.claude/projects/-opt-openAi-DraftRight/memory/` capturing:
- Resend account is configured for `draftright.info`
- Email verification is **soft-gate** for now (banner, not block)
- Free plan = 20/day in prod (not unlimited)
- Pre-filled credentials removed from all clients

- [ ] **Step 3: Tag the release**

```bash
git tag -a v2.2.0 -m "Public registration with email verification"
git push origin v2.2.0  # if remote configured
```

---

## Self-Review Results

**Spec coverage:**
- ✅ Email + password registration → Tasks 5, 8
- ✅ Email verification (OTP) → Tasks 1-7, 9
- ✅ Social login → unchanged, already shipped
- ✅ Web signup page → Tasks 8-11
- ✅ Client deep-links → Tasks 12-13
- ✅ Free plan limit → Task 14
- ✅ Pre-fill removal → Tasks 12-13

**Placeholders:** none — every step has concrete code or commands.

**Type consistency:** `email_verified`, `email_verification_code`, `email_verification_expires` used consistently across entity, service, controller, frontend.

**Known gap:** Soft-gate banner UI (showing "verify your email" inside clients) is **not** in this plan. The verification status is returned in the `/auth/me` response (visible to clients) but no UI consumes it yet. That's a Phase 2 polish — track separately.

---

## Execution Handoff

Plan saved to `docs/superpowers/plans/2026-04-30-customer-registration.md`.

Two execution options:

1. **Subagent-Driven (recommended)** — I dispatch a fresh subagent per task, review between tasks, fast iteration.
2. **Inline Execution** — Execute tasks in this session using executing-plans, batch with checkpoints for review.

Which approach?
