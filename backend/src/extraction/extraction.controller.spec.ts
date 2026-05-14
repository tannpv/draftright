import { Test } from '@nestjs/testing';
import { INestApplication, ValidationPipe } from '@nestjs/common';
import * as request from 'supertest';
import { ExtractionController } from './extraction.controller';
import { ExtractionService } from './extraction.service';
import { JwtAuthGuard } from '../auth/jwt-auth.guard';

describe('ExtractionController (e2e)', () => {
  let app: INestApplication;
  let service: { extract: jest.Mock };

  beforeAll(async () => {
    service = {
      extract: jest.fn().mockResolvedValue({
        entities: [
          { kind: 'address', value: '123 Lê Lợi', display: '123 Lê Lợi', start: 0, end: 10, confidence: 0.8 },
        ],
        provider: 'openai',
        tokensUsed: 50,
      }),
    };
    const mod = await Test.createTestingModule({
      controllers: [ExtractionController],
      providers: [{ provide: ExtractionService, useValue: service }],
    })
      .overrideGuard(JwtAuthGuard)
      .useValue({ canActivate: () => true })
      .compile();
    app = mod.createNestApplication();
    app.useGlobalPipes(new ValidationPipe({ whitelist: true, transform: true }));
    await app.init();
  });

  afterAll(async () => app.close());

  it('200 with valid body', async () => {
    const resp = await request(app.getHttpServer())
      .post('/extract')
      .send({ text: '123 Lê Lợi' });
    expect(resp.status).toBe(200);
    expect(resp.body.entities).toHaveLength(1);
  });

  it('400 when text is missing', async () => {
    const resp = await request(app.getHttpServer())
      .post('/extract')
      .send({});
    expect(resp.status).toBe(400);
  });

  it('400 when text exceeds 8000 chars', async () => {
    const resp = await request(app.getHttpServer())
      .post('/extract')
      .send({ text: 'a'.repeat(8001) });
    expect(resp.status).toBe(400);
  });
});
