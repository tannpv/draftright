import { Injectable } from '@nestjs/common';
import { InjectRepository } from '@nestjs/typeorm';
import { Repository, MoreThanOrEqual } from 'typeorm';
import { RewriteLog } from './entities/rewrite-log.entity';

@Injectable()
export class RewriteLogService {
  constructor(
    @InjectRepository(RewriteLog)
    private readonly rewriteLogRepo: Repository<RewriteLog>,
  ) {}

  async log(data: {
    tone: string;
    input_text: string;
    output_text: string;
    model: string;
    provider_type: string;
    response_time_ms: number;
  }): Promise<void> {
    const entry = this.rewriteLogRepo.create(data);
    await this.rewriteLogRepo.save(entry);
  }

  async count(): Promise<number> {
    return this.rewriteLogRepo.count();
  }

  async countByQuality(): Promise<{ pending: number; approved: number; rejected: number }> {
    const [pending, approved, rejected] = await Promise.all([
      this.rewriteLogRepo.count({ where: { quality: 'pending' } }),
      this.rewriteLogRepo.count({ where: { quality: 'approved' } }),
      this.rewriteLogRepo.count({ where: { quality: 'rejected' } }),
    ]);
    return { pending, approved, rejected };
  }

  async findPending(page = 1, limit = 20): Promise<{ logs: RewriteLog[]; total: number }> {
    const [logs, total] = await this.rewriteLogRepo.findAndCount({
      where: { quality: 'pending' },
      order: { created_at: 'DESC' },
      skip: (page - 1) * limit,
      take: limit,
    });
    return { logs, total };
  }

  async updateQuality(id: string, quality: 'approved' | 'rejected'): Promise<void> {
    await this.rewriteLogRepo.update(id, { quality });
  }

  async exportApproved(format: 'jsonl'): Promise<string> {
    const logs = await this.rewriteLogRepo.find({
      where: { quality: 'approved' },
      order: { created_at: 'ASC' },
    });

    return logs.map(log => JSON.stringify({
      messages: [
        { role: 'system', content: `Rewrite the following text in a ${log.tone} tone. Return only the rewritten text.` },
        { role: 'user', content: log.input_text },
        { role: 'assistant', content: log.output_text },
      ],
    })).join('\n');
  }

  async exportAll(format: 'jsonl'): Promise<string> {
    const logs = await this.rewriteLogRepo.find({
      where: { quality: 'approved' },
      order: { created_at: 'ASC' },
    });

    // Ollama fine-tune format
    return logs.map(log => JSON.stringify({
      input: log.input_text,
      output: log.output_text,
      tone: log.tone,
      model: log.model,
    })).join('\n');
  }
}
