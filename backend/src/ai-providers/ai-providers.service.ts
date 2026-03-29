import { Injectable, BadRequestException } from '@nestjs/common';
import { InjectRepository } from '@nestjs/typeorm';
import { Repository } from 'typeorm';
import { AiProvider } from './entities/ai-provider.entity';

@Injectable()
export class AiProvidersService {
  constructor(
    @InjectRepository(AiProvider)
    private readonly providersRepo: Repository<AiProvider>,
  ) {}

  async findAll(): Promise<AiProvider[]> {
    return this.providersRepo.find({ order: { created_at: 'ASC' } });
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
    const provider = this.providersRepo.create(data);
    return this.providersRepo.save(provider);
  }

  async update(id: string, data: Partial<AiProvider>): Promise<AiProvider> {
    if (data.is_default) {
      await this.providersRepo.update({}, { is_default: false });
    }
    await this.providersRepo.update(id, data);
    return this.providersRepo.findOneOrFail({ where: { id } });
  }

  async softDelete(id: string): Promise<void> {
    await this.providersRepo.update(id, { is_active: false, is_default: false });
  }

  async callProvider(provider: AiProvider, systemPrompt: string, userText: string): Promise<{ text: string; responseTimeMs: number }> {
    const startTime = Date.now();

    const body = {
      model: provider.model,
      temperature: Number(provider.temperature),
      messages: [
        { role: 'system', content: systemPrompt },
        { role: 'user', content: userText },
      ],
    };

    const headers: Record<string, string> = { 'Content-Type': 'application/json' };
    if (provider.api_key) {
      headers['Authorization'] = `Bearer ${provider.api_key}`;
    }

    const response = await fetch(provider.endpoint_url, {
      method: 'POST',
      headers,
      body: JSON.stringify(body),
    });

    if (!response.ok) {
      const errorText = await response.text();
      throw new Error(`AI provider error (${response.status}): ${errorText}`);
    }

    const json = await response.json();
    const text = json.choices?.[0]?.message?.content?.trim() || '';
    const responseTimeMs = Date.now() - startTime;

    return { text, responseTimeMs };
  }
}
