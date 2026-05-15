import { Test } from '@nestjs/testing';
import { ExtractionService } from './extraction.service';
import { AiProvidersService } from '../ai-providers/ai-providers.service';
import { EntityKind } from './dto/extract.dto';

describe('ExtractionService', () => {
  let service: ExtractionService;
  let aiProviders: { findDefault: jest.Mock; callProvider: jest.Mock };

  beforeEach(async () => {
    aiProviders = {
      findDefault: jest.fn().mockResolvedValue({
        id: 'p1',
        type: 'openai',
        name: 'openai',
        model: 'gpt-4o-mini',
        is_active: true,
        endpoint_url: 'http://x',
        api_key: 'x',
        temperature: 0.2,
      }),
      callProvider: jest.fn(),
    };
    const mod = await Test.createTestingModule({
      providers: [
        ExtractionService,
        { provide: AiProvidersService, useValue: aiProviders },
      ],
    }).compile();
    service = mod.get(ExtractionService);
  });

  it('returns empty when LLM responds with non-JSON', async () => {
    aiProviders.callProvider.mockResolvedValue({ text: 'not json', responseTimeMs: 5 });
    const out = await service.extract('hello world');
    expect(out.entities).toEqual([]);
  });

  it('drops entities whose value is not in original text (hallucination guard)', async () => {
    aiProviders.callProvider.mockResolvedValue({
      text: JSON.stringify([
        { kind: 'address', value: '123 Lê Lợi', display: '123 Lê Lợi', confidence: 0.8 },
        { kind: 'address', value: 'FAKE STREET', display: 'FAKE STREET', confidence: 0.8 },
      ]),
      responseTimeMs: 10,
    });
    const out = await service.extract('Địa chỉ 123 Lê Lợi');
    expect(out.entities).toHaveLength(1);
    expect(out.entities[0].value).toBe('123 Lê Lợi');
  });

  it('recomputes offsets via indexOf', async () => {
    aiProviders.callProvider.mockResolvedValue({
      text: JSON.stringify([
        { kind: 'address', value: '123 Lê Lợi', display: '123 Lê Lợi', confidence: 0.8, start: 999, end: 1010 },
      ]),
      responseTimeMs: 10,
    });
    const out = await service.extract('Địa chỉ 123 Lê Lợi');
    expect(out.entities[0].start).toBe(8);
    expect(out.entities[0].end).toBe(8 + '123 Lê Lợi'.length);
  });

  it('drops disallowed kinds (regex-handled set)', async () => {
    aiProviders.callProvider.mockResolvedValue({
      text: JSON.stringify([
        { kind: 'phone', value: '0912345678', display: '0912345678', confidence: 0.9 },
        { kind: 'address', value: '123 Lê Lợi', display: '123 Lê Lợi', confidence: 0.8 },
      ]),
      responseTimeMs: 10,
    });
    const out = await service.extract('phone 0912345678 at 123 Lê Lợi');
    const kinds = out.entities.map((e) => e.kind);
    expect(kinds).not.toContain('phone');
    expect(kinds).toContain('address');
  });
});
