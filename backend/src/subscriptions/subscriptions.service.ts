import { Injectable } from '@nestjs/common';
import { InjectRepository } from '@nestjs/typeorm';
import { Repository } from 'typeorm';
import { Subscription, SubscriptionStatus, StoreType } from './entities/subscription.entity';

@Injectable()
export class SubscriptionsService {
  constructor(
    @InjectRepository(Subscription)
    private readonly subsRepo: Repository<Subscription>,
  ) {}

  async findActiveByUserId(userId: string): Promise<Subscription | null> {
    return this.subsRepo.findOne({
      where: { user_id: userId, status: SubscriptionStatus.ACTIVE },
      relations: ['plan'],
      order: { created_at: 'DESC' },
    });
  }

  async createFreeSubscription(userId: string, planId: string): Promise<Subscription> {
    const sub = this.subsRepo.create({
      user_id: userId,
      plan_id: planId,
      status: SubscriptionStatus.ACTIVE,
      store_type: StoreType.ADMIN_GRANTED,
      started_at: new Date(),
      expires_at: null,
    });
    return this.subsRepo.save(sub);
  }

  async grant(userId: string, planId: string, expiresAt?: Date): Promise<Subscription> {
    await this.subsRepo.update(
      { user_id: userId, status: SubscriptionStatus.ACTIVE },
      { status: SubscriptionStatus.CANCELLED },
    );
    const sub = this.subsRepo.create({
      user_id: userId,
      plan_id: planId,
      status: SubscriptionStatus.ACTIVE,
      store_type: StoreType.ADMIN_GRANTED,
      started_at: new Date(),
      expires_at: expiresAt || null,
    });
    return this.subsRepo.save(sub);
  }

  async verifyAndActivate(
    userId: string, planId: string, storeType: StoreType, transactionId: string, expiresAt: Date,
  ): Promise<Subscription> {
    await this.subsRepo.update(
      { user_id: userId, status: SubscriptionStatus.ACTIVE },
      { status: SubscriptionStatus.CANCELLED },
    );
    const sub = this.subsRepo.create({
      user_id: userId, plan_id: planId, status: SubscriptionStatus.ACTIVE,
      store_type: storeType, store_transaction_id: transactionId,
      started_at: new Date(), expires_at: expiresAt,
    });
    return this.subsRepo.save(sub);
  }

  async countActive(): Promise<number> {
    return this.subsRepo.count({ where: { status: SubscriptionStatus.ACTIVE } });
  }

  async findAllPaginated(options: { search?: string; page?: number; limit?: number }): Promise<{ subscriptions: Subscription[]; total: number }> {
    const { search, page = 1, limit = 20 } = options;
    const qb = this.subsRepo.createQueryBuilder('sub')
      .leftJoinAndSelect('sub.user', 'user')
      .leftJoinAndSelect('sub.plan', 'plan')
      .orderBy('sub.created_at', 'DESC')
      .skip((page - 1) * limit)
      .take(limit);

    if (search) {
      qb.where('user.email ILIKE :search OR user.name ILIKE :search', { search: `%${search}%` });
    }

    const [subscriptions, total] = await qb.getManyAndCount();
    return { subscriptions, total };
  }

  async getPlansBreakdown(): Promise<{ plan_name: string; active_count: number; price_cents: number }[]> {
    const result = await this.subsRepo.createQueryBuilder('sub')
      .leftJoin('sub.plan', 'plan')
      .select('plan.name', 'plan_name')
      .addSelect('plan.price_cents', 'price_cents')
      .addSelect('COUNT(*)', 'active_count')
      .where('sub.status = :status', { status: 'active' })
      .groupBy('plan.name')
      .addGroupBy('plan.price_cents')
      .getRawMany();
    return result.map(r => ({
      plan_name: r.plan_name,
      active_count: parseInt(r.active_count),
      price_cents: parseInt(r.price_cents) || 0,
    }));
  }

  async getMonthlyStats(months: number = 12): Promise<{ month: string; new_subscriptions: number; revenue_cents: number; churned: number }[]> {
    const stats: { month: string; new_subscriptions: number; revenue_cents: number; churned: number }[] = [];
    const now = new Date();

    for (let i = months - 1; i >= 0; i--) {
      const date = new Date(now.getFullYear(), now.getMonth() - i, 1);
      const monthStart = new Date(date.getFullYear(), date.getMonth(), 1);
      const monthEnd = new Date(date.getFullYear(), date.getMonth() + 1, 1);
      const monthStr = `${date.getFullYear()}-${String(date.getMonth() + 1).padStart(2, '0')}`;

      const newSubs = await this.subsRepo.createQueryBuilder('sub')
        .leftJoin('sub.plan', 'plan')
        .where('sub.started_at >= :start AND sub.started_at < :end', { start: monthStart, end: monthEnd })
        .getCount();

      const revenueResult = await this.subsRepo.createQueryBuilder('sub')
        .leftJoin('sub.plan', 'plan')
        .select('COALESCE(SUM(plan.price_cents), 0)', 'total')
        .where('sub.started_at >= :start AND sub.started_at < :end', { start: monthStart, end: monthEnd })
        .getRawOne();

      const churned = await this.subsRepo.createQueryBuilder('sub')
        .where('sub.status IN (:...statuses)', { statuses: ['cancelled', 'expired'] })
        .andWhere('sub.updated_at >= :start AND sub.updated_at < :end', { start: monthStart, end: monthEnd })
        .getCount();

      stats.push({
        month: monthStr,
        new_subscriptions: newSubs,
        revenue_cents: parseInt(revenueResult?.total) || 0,
        churned,
      });
    }

    return stats;
  }
}
