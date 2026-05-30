import { Injectable, BadRequestException } from '@nestjs/common';
import { InjectRepository } from '@nestjs/typeorm';
import { Repository } from 'typeorm';
import { AiProvider } from './entities/ai-provider.entity';
import { ListQuery, ListResult, applyListQuery } from '../common/list-query';
import { ProviderStrategyRegistry } from './strategies/provider-strategy.registry';

@Injectable()
export class AiProvidersService {
  constructor(
    @InjectRepository(AiProvider)
    private readonly providersRepo: Repository<AiProvider>,
    private readonly strategyRegistry: ProviderStrategyRegistry,
  ) {}

  async findAll(): Promise<AiProvider[]> {
    return this.providersRepo.find({ order: { created_at: 'ASC' } });
  }

  async findAllPaginated(query: ListQuery): Promise<ListResult<AiProvider>> {
    const qb = this.providersRepo.createQueryBuilder('provider');
    return applyListQuery(
      qb,
      query,
      ['provider.name', 'provider.type', 'provider.model'],
      {
        name: 'provider.name',
        type: 'provider.type',
        model: 'provider.model',
        is_default: 'provider.is_default',
        is_active: 'provider.is_active',
        created_at: 'provider.created_at',
      },
      'provider.created_at',
      'provider.is_active',
    );
  }

  async findById(id: string): Promise<AiProvider | null> {
    return this.providersRepo.findOne({ where: { id } });
  }

  async findDefault(): Promise<AiProvider> {
    const provider = await this.providersRepo.findOne({ where: { is_default: true, is_active: true } });
    if (!provider) throw new BadRequestException('No default AI provider configured');
    return provider;
  }

  async create(data: Partial<AiProvider>): Promise<AiProvider> {
    // Single-default invariant: at most one row may have is_default=true.
    // update() already enforces this; mirror the demotion here so the
    // "create a new provider with Default ✓" admin flow can't end up
    // with two defaults (bug report 1332824a, 2026-05-30).
    if (data.is_default) {
      await this.providersRepo
        .createQueryBuilder()
        .update()
        .set({ is_default: false })
        .where('is_default = :val', { val: true })
        .execute();
    }
    const provider = this.providersRepo.create(data);
    return this.providersRepo.save(provider);
  }

  async update(id: string, data: Partial<AiProvider>): Promise<AiProvider> {
    if (data.is_default) {
      await this.providersRepo
        .createQueryBuilder()
        .update()
        .set({ is_default: false })
        .where('is_default = :val', { val: true })
        .execute();
    }
    await this.providersRepo.update(id, data);
    return this.providersRepo.findOneOrFail({ where: { id } });
  }

  async softDelete(id: string): Promise<void> {
    await this.providersRepo.update(id, { is_active: false, is_default: false });
  }

  /**
   * Dispatches the rewrite request to whichever provider strategy
   * claims the config. Wire-level concerns (request body shape, auth
   * headers, transient 429 retry, response decoding) all live inside
   * the strategy classes — this method stays a thin router.
   *
   * Adding a new upstream API surface = new strategy file + one entry
   * in the registry factory. Zero edits here.
   */
  async callProvider(
    provider: AiProvider,
    systemPrompt: string,
    userText: string,
  ): Promise<{ text: string; responseTimeMs: number }> {
    const startTime = Date.now();
    const strategy = this.strategyRegistry.pick(provider);
    return strategy.call(provider, systemPrompt, userText, startTime);
  }
}
