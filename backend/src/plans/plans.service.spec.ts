import { Test } from '@nestjs/testing';
import { getRepositoryToken } from '@nestjs/typeorm';
import { PlansService } from './plans.service';
import { Plan } from './entities/plan.entity';

describe('PlansService.findPublic', () => {
  const find = jest.fn();
  let service: PlansService;

  beforeEach(async () => {
    find.mockReset();
    const mod = await Test.createTestingModule({
      providers: [
        PlansService,
        { provide: getRepositoryToken(Plan), useValue: { find } },
      ],
    }).compile();
    service = mod.get(PlansService);
  });

  it('returns only active plans, cheapest first', async () => {
    const rows = [{ id: 'free' }, { id: 'pro' }];
    find.mockResolvedValue(rows);

    const result = await service.findPublic();

    expect(result).toBe(rows);
    expect(find).toHaveBeenCalledWith({
      where: { is_active: true },
      order: { price_cents: 'ASC' },
    });
  });
});
