import { Injectable } from '@nestjs/common';
import { InjectRepository } from '@nestjs/typeorm';
import { Repository } from 'typeorm';
import { Plan, BillingPeriod } from './entities/plan.entity';

@Injectable()
export class PlansService {
  constructor(
    @InjectRepository(Plan)
    private readonly plansRepo: Repository<Plan>,
  ) {}

  async findAll(): Promise<Plan[]> {
    return this.plansRepo.find({ order: { created_at: 'ASC' } });
  }

  async findById(id: string): Promise<Plan | null> {
    return this.plansRepo.findOne({ where: { id } });
  }

  async findFreePlan(): Promise<Plan> {
    const plan = await this.plansRepo.findOne({ where: { billing_period: BillingPeriod.NONE, is_active: true } });
    if (!plan) throw new Error('Free plan not found. Run seed first.');
    return plan;
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
