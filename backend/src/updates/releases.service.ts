import { Injectable } from '@nestjs/common';
import { InjectRepository } from '@nestjs/typeorm';
import { Repository } from 'typeorm';
import { AppRelease } from './entities/app-release.entity';

export type Platform = 'mac' | 'windows' | 'linux' | 'android' | 'ios';

@Injectable()
export class ReleasesService {
  constructor(
    @InjectRepository(AppRelease)
    private readonly repo: Repository<AppRelease>,
  ) {}

  /** Returns all releases keyed by platform. Empty map if table is empty. */
  async listAll(): Promise<Record<Platform, AppRelease | null>> {
    const rows = await this.repo.find();
    const out: Record<string, AppRelease | null> = {
      mac: null, windows: null, linux: null, android: null, ios: null,
    };
    for (const row of rows) out[row.platform] = row;
    return out as Record<Platform, AppRelease | null>;
  }

  async getOne(platform: string): Promise<AppRelease | null> {
    return this.repo.findOne({ where: { platform } });
  }

  /** Insert or update a release row. Returns the persisted record. */
  async upsert(input: {
    platform: string;
    version: string;
    download_url: string;
    release_notes?: string;
    required?: boolean;
  }): Promise<AppRelease> {
    const existing = await this.repo.findOne({ where: { platform: input.platform } });
    if (existing) {
      existing.version = input.version;
      existing.download_url = input.download_url;
      if (input.release_notes !== undefined) existing.release_notes = input.release_notes;
      if (input.required !== undefined) existing.required = input.required;
      return this.repo.save(existing);
    }
    const row = this.repo.create({
      platform: input.platform,
      version: input.version,
      download_url: input.download_url,
      release_notes: input.release_notes ?? '',
      required: input.required ?? false,
    });
    return this.repo.save(row);
  }
}
