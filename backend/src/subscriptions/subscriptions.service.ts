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
}
