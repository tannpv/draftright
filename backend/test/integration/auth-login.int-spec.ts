import { Test, TestingModule } from '@nestjs/testing';
import { INestApplication, ValidationPipe } from '@nestjs/common';
import { getRepositoryToken } from '@nestjs/typeorm';
import { Repository } from 'typeorm';
import * as request from 'supertest';
import { AppModule } from '../../src/app.module';
import { AllExceptionsFilter } from '../../src/common/all-exceptions.filter';
import { Plan, BillingPeriod } from '../../src/plans/entities/plan.entity';

/**
 * Integration coverage for the registration → login → /auth/me loop.
 *
 * Drives the actual NestJS HTTP layer (via supertest) against the
 * containerised Postgres + Redis boot in globalSetup. Every assertion
 * is on the wire shape clients depend on — `access_token`,
 * `refresh_token`, claims, `flags.use_go_backend`, the error envelope.
 *
 * Catches the class of bugs unit tests can't see:
 *   - DTO validation drifting (e.g. password min-length change).
 *   - JWT secret rotation breaking sign-then-verify round-trip.
 *   - /auth/me returning a different shape than the
 *     macOS / Flutter / Swift clients expect.
 *   - Error envelope regression (missing `code` or `request_id`).
 */
describe('Auth register + login + /auth/me (integration)', () => {
  let app: INestApplication;
  const uniqueEmail = `it-${Date.now()}-${Math.floor(Math.random() * 1e6)}@draftright.test`;
  const password = 'integration-password-123';
  let accessToken = '';

  beforeAll(async () => {
    const mod: TestingModule = await Test.createTestingModule({
      imports: [AppModule],
    }).compile();

    app = mod.createNestApplication();
    // Mirror main.ts wiring so wire shape under test matches prod.
    app.useGlobalPipes(new ValidationPipe({ whitelist: true, transform: true }));
    app.useGlobalFilters(new AllExceptionsFilter());
    await app.init();

    // Register-path attaches a free subscription, which requires a
    // Free plan row.  Seed a minimal one — production gets this via
    // backend/src/seed.ts; integration tests roll their own so the
    // suite is self-contained.
    const plansRepo: Repository<Plan> =
      app.get(getRepositoryToken(Plan));
    const existing = await plansRepo.findOne({
      where: { billing_period: BillingPeriod.NONE, is_active: true },
    });
    if (!existing) {
      await plansRepo.save(
        plansRepo.create({
          name: 'Free',
          price_cents: 0,
          billing_period: BillingPeriod.NONE,
          daily_limit: 100,
          is_active: true,
        }),
      );
    }
  });

  afterAll(async () => {
    await app?.close();
  });

  it('POST /auth/register → 201 + access + refresh', async () => {
    const res = await request(app.getHttpServer())
      .post('/auth/register')
      .send({ email: uniqueEmail, password, name: 'Integration Tester' })
      .expect(201);

    expect(res.body).toHaveProperty('access_token');
    expect(res.body).toHaveProperty('refresh_token');
    expect(typeof res.body.access_token).toBe('string');
    expect(res.body.access_token.length).toBeGreaterThan(40);
  });

  it('POST /auth/login with the same credentials → 200/201 + tokens', async () => {
    const res = await request(app.getHttpServer())
      .post('/auth/login')
      .send({ email: uniqueEmail, password });

    expect([200, 201]).toContain(res.status);
    expect(res.body).toHaveProperty('access_token');
    expect(res.body).toHaveProperty('refresh_token');
    accessToken = res.body.access_token;
  });

  it('POST /auth/login with wrong password → 401 + error envelope', async () => {
    const res = await request(app.getHttpServer())
      .post('/auth/login')
      .send({ email: uniqueEmail, password: 'completely-wrong-pw' })
      .expect(401);

    // Standard envelope (S9): { error, code, request_id }.
    expect(res.body).toHaveProperty('error');
    expect(res.body).toHaveProperty('code');
    expect(res.body).toHaveProperty('request_id');
    expect(res.body.code).toBe('invalid-token');
  });

  it('POST /auth/register with bad email → 400 invalid-input', async () => {
    const res = await request(app.getHttpServer())
      .post('/auth/register')
      .send({ email: 'not-an-email', password, name: 'X' })
      .expect(400);

    expect(res.body.code).toBe('invalid-input');
    expect(res.body).toHaveProperty('request_id');
  });

  it('GET /auth/me with valid token → user + flags envelope', async () => {
    const res = await request(app.getHttpServer())
      .get('/auth/me')
      .set('Authorization', `Bearer ${accessToken}`)
      .expect(200);

    expect(res.body).toHaveProperty('id');
    expect(res.body).toHaveProperty('email', uniqueEmail);
    expect(res.body).toHaveProperty('role');
    expect(res.body).toHaveProperty('flags');
    expect(res.body.flags).toHaveProperty('use_go_backend');
    expect(typeof res.body.flags.use_go_backend).toBe('boolean');
  });

  it('GET /auth/me without token → 401 invalid-token envelope', async () => {
    const res = await request(app.getHttpServer())
      .get('/auth/me')
      .expect(401);

    expect(res.body.code).toBe('invalid-token');
    expect(res.body).toHaveProperty('request_id');
  });

  it('X-Request-Id round-trips through the response (S11)', async () => {
    const rid = `it-correlation-${Date.now()}`;
    const res = await request(app.getHttpServer())
      .get('/auth/me')
      .set('X-Request-Id', rid)
      .expect(401);

    expect(res.headers['x-request-id']).toBe(rid);
    expect(res.body.request_id).toBe(rid);
  });
});
