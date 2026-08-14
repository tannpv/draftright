import { Injectable, Logger } from '@nestjs/common';
import { InjectRepository } from '@nestjs/typeorm';
import { Repository } from 'typeorm';
import { UserContext } from './entities/user-context.entity';
import { UpdateUserContextDto } from './dto/update-user-context.dto';
import { buildContextPreamble } from './user-context.constants';

@Injectable()
export class UserContextService {
  private readonly logger = new Logger(UserContextService.name);

  constructor(
    @InjectRepository(UserContext)
    private readonly repo: Repository<UserContext>,
  ) {}

  /** The user's own context, or null if they have never set one. */
  async get(userId: string): Promise<UserContext | null> {
    return this.repo.findOne({ where: { user_id: userId } });
  }

  /**
   * Create-or-update the caller's context. Only the fields present in the DTO
   * change; the rest keep their stored value (or the entity default on first
   * write). Returns the saved row.
   */
  async upsert(userId: string, dto: UpdateUserContextDto): Promise<UserContext> {
    const existing = await this.get(userId);
    const row = existing ?? this.repo.create({ user_id: userId });
    if (dto.enabled !== undefined) row.enabled = dto.enabled;
    if (dto.job_title !== undefined) row.job_title = dto.job_title;
    if (dto.industry !== undefined) row.industry = dto.industry;
    if (dto.audience !== undefined) row.audience = dto.audience;
    if (dto.style_notes !== undefined) row.style_notes = dto.style_notes;
    return this.repo.save(row);
  }

  /** Hard-delete the caller's context (GDPR erasure — not a flag flip). */
  async clear(userId: string): Promise<void> {
    await this.repo.delete({ user_id: userId });
  }

  /**
   * The system-context preamble to prepend to the rewrite prompt for this user,
   * or null when there is nothing to inject (no row, disabled, or empty). Never
   * throws into the rewrite path — a lookup failure degrades to no
   * personalization rather than a failed rewrite.
   */
  async getPreamble(userId: string): Promise<string | null> {
    try {
      const ctx = await this.get(userId);
      if (!ctx) return null;
      return buildContextPreamble(ctx);
    } catch (err) {
      this.logger.warn(`context preamble lookup failed for ${userId}: ${err}`);
      return null;
    }
  }
}
