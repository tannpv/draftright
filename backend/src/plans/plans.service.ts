import { Injectable } from '@nestjs/common';
import { InjectRepository } from '@nestjs/typeorm';
import { Repository } from 'typeorm';
import { Plan, BillingPeriod } from './entities/plan.entity';
import { ListQuery, ListResult, applyListQuery } from '../common/list-query';

@Injectable()
export class PlansService {
  constructor(
    @InjectRepository(Plan)
    private readonly plansRepo: Repository<Plan>,
  ) {}

  async findAll(): Promise<Plan[]> {
    return this.plansRepo.find({ order: { created_at: 'ASC' } });
  }

  /**
   * Find the first active plan matching the supplied `(billing_period,
   * currency)` pair.  Used by the LS webhook handler to map a
   * received variant → matching local plan when the user switches
   * variants on the hosted checkout page.  Returns null when nothing
   * matches.
   */
  async findFirstActive(criteria: { billing_period: string; currency: string }): Promise<Plan | null> {
    return this.plansRepo.findOne({
      where: {
        is_active: true,
        billing_period: criteria.billing_period as BillingPeriod,
        currency: criteria.currency,
      },
      order: { created_at: 'ASC' },
    });
  }

  /**
   * Active plans for the public website/app pricing + checkout, cheapest first.
   * The client fetches these instead of hard-coding plan IDs, so prices and
   * plan changes made in admin never go stale on the storefront.
   */
  async findPublic(): Promise<Plan[]> {
    return this.plansRepo.find({
      where: { is_active: true },
      order: { price_cents: 'ASC' },
    });
  }

  async findAllPaginated(query: ListQuery): Promise<ListResult<Plan>> {
    const qb = this.plansRepo.createQueryBuilder('plan');
    return applyListQuery(
      qb,
      query,
      ['plan.name', 'plan.currency', 'plan.billing_period'],
      {
        name: 'plan.name',
        price: 'plan.price_cents',
        currency: 'plan.currency',
        billing_period: 'plan.billing_period',
        trial_days: 'plan.trial_days',
        is_active: 'plan.is_active',
        created_at: 'plan.created_at',
      },
      'plan.created_at',
      'plan.is_active',
    );
  }

  async findById(id: string): Promise<Plan | null> {
    return this.plansRepo.findOne({ where: { id } });
  }

  async findFreePlan(): Promise<Plan> {
    const plan = await this.plansRepo.findOne({ where: { billing_period: BillingPeriod.NONE, is_active: true } });
    if (!plan) throw new Error('Free plan not found. Run seed first.');
    return plan;
  }

  async findByName(name: string): Promise<Plan | null> {
    return this.plansRepo.findOne({ where: { name, is_active: true } });
  }

  async create(data: Partial<Plan>): Promise<Plan> {
    const plan = this.plansRepo.create(data);
    return this.plansRepo.save(plan);
  }

  async update(id: string, data: Partial<Plan>): Promise<Plan> {
    await this.plansRepo.update(id, data);
    return this.plansRepo.findOneOrFail({ where: { id } });
  }

  async softDelete(id: string): Promise<void> {
    await this.plansRepo.update(id, { is_active: false });
  }
}
