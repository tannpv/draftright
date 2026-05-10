import { Injectable, BadRequestException } from '@nestjs/common';
import { InjectRepository } from '@nestjs/typeorm';
import { Repository } from 'typeorm';
import { AppReleasePolicy } from './entities/app-release-policy.entity';

const PLATFORMS = ['mac', 'windows', 'linux', 'android', 'ios'] as const;
const CHANNELS = ['direct', 'store'] as const;
const STATUSES = ['not_submitted', 'in_review', 'approved', 'rejected', 'n/a'] as const;

@Injectable()
export class PoliciesService {
  constructor(
    @InjectRepository(AppReleasePolicy)
    private readonly repo: Repository<AppReleasePolicy>,
  ) {}

  async upsert(input: {
    platform: string;
    preferred?: string;
    store_status?: string;
    notes?: string;
  }): Promise<AppReleasePolicy> {
    if (!PLATFORMS.includes(input.platform as typeof PLATFORMS[number])) {
      throw new BadRequestException(`platform must be one of: ${PLATFORMS.join(', ')}`);
    }
    if (input.preferred !== undefined && !CHANNELS.includes(input.preferred as typeof CHANNELS[number])) {
      throw new BadRequestException(`preferred must be one of: ${CHANNELS.join(', ')}`);
    }
    if (input.store_status !== undefined && !STATUSES.includes(input.store_status as typeof STATUSES[number])) {
      throw new BadRequestException(`store_status must be one of: ${STATUSES.join(', ')}`);
    }
    const existing = await this.repo.findOne({ where: { platform: input.platform } });
    if (existing) {
      if (input.preferred !== undefined) existing.preferred = input.preferred;
      if (input.store_status !== undefined) existing.store_status = input.store_status;
      if (input.notes !== undefined) existing.notes = input.notes;
      return this.repo.save(existing);
    }
    const row = this.repo.create({
      platform: input.platform,
      preferred: input.preferred ?? 'direct',
      store_status: input.store_status ?? 'not_submitted',
      notes: input.notes ?? '',
    });
    return this.repo.save(row);
  }
}
