import { Injectable } from '@nestjs/common';
import { InjectRepository } from '@nestjs/typeorm';
import { Repository, MoreThanOrEqual } from 'typeorm';
import { UsageLog } from './entities/usage-log.entity';

@Injectable()
export class UsageService {
  constructor(
    @InjectRepository(UsageLog)
    private readonly usageRepo: Repository<UsageLog>,
  ) {}

  async countTodayByUser(userId: string): Promise<number> {
    const todayStart = new Date();
    todayStart.setHours(0, 0, 0, 0);
    return this.usageRepo.count({
      where: { user_id: userId, created_at: MoreThanOrEqual(todayStart) },
    });
  }

  async log(data: {
    user_id: string; tone: string; input_length: number; output_length: number;
    ai_provider_id: string; response_time_ms: number;
  }): Promise<UsageLog> {
    const entry = this.usageRepo.create(data);
    return this.usageRepo.save(entry);
  }

  async countToday(): Promise<number> {
    const todayStart = new Date();
    todayStart.setHours(0, 0, 0, 0);
    return this.usageRepo.count({ where: { created_at: MoreThanOrEqual(todayStart) } });
  }

  async countThisMonth(): Promise<number> {
    const monthStart = new Date();
    monthStart.setDate(1);
    monthStart.setHours(0, 0, 0, 0);
    return this.usageRepo.count({ where: { created_at: MoreThanOrEqual(monthStart) } });
  }

  async findRecentByUser(userId: string, limit: number = 20): Promise<UsageLog[]> {
    return this.usageRepo.find({
      where: { user_id: userId },
      order: { created_at: 'DESC' },
      take: limit,
    });
  }
}
