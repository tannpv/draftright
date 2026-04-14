import { Injectable, BadRequestException } from '@nestjs/common';
import { InjectRepository } from '@nestjs/typeorm';
import { Repository } from 'typeorm';
import { AiProvider, AiProviderType } from './entities/ai-provider.entity';

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
      await this.providersRepo
        .createQueryBuilder()
        .update()
        .set({ is_default: false })
        .where('is_default = :val', { val: true })
        .execute();
    }
    await this.providersRepo.update(id, data);
    return this.providersRepo.findOneOrFail({ where: { id } });
  }

  async softDelete(id: string): Promise<void> {
    await this.providersRepo.update(id, { is_active: false, is_default: false });
  }

  async callProvider(provider: AiProvider, systemPrompt: string, userText: string): Promise<{ text: string; responseTimeMs: number }> {
    const startTime = Date.now();

    if (provider.type === AiProviderType.ANTHROPIC) {
      return this.callAnthropic(provider, systemPrompt, userText, startTime);
    }

    // For smaller local models (Ollama), embed instructions in user message
    const isLocal = provider.type === AiProviderType.OLLAMA;
    const messages = isLocal
      ? [
          { role: 'system', content: 'You are a writing assistant. If the user provides existing text, rewrite it as instructed. If the user provides a request or instruction, generate the requested content. You ONLY output the result text. Never answer questions about yourself, never explain your process, never add commentary. CRITICAL: You must reply in the SAME LANGUAGE as the input text. If the input is Vietnamese, reply in Vietnamese. If French, reply in French. Never switch to English unless the input is in English.' },
          { role: 'user', content: `${systemPrompt}\n\nIMPORTANT: Your response MUST be in the same language as the text below.\n\nUser input:\n"${userText}"\n\nOutput:` },
        ]
      : [
          { role: 'system', content: systemPrompt },
          { role: 'user', content: userText },
        ];

    const body = {
      model: provider.model,
      temperature: Number(provider.temperature),
      messages,
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

  private async callAnthropic(provider: AiProvider, systemPrompt: string, userText: string, startTime: number): Promise<{ text: string; responseTimeMs: number }> {
    const body = {
      model: provider.model,
      max_tokens: 1024,
      system: systemPrompt,
      messages: [
        { role: 'user', content: userText },
      ],
    };

    const response = await fetch(provider.endpoint_url, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        'x-api-key': provider.api_key,
        'anthropic-version': '2023-06-01',
      },
      body: JSON.stringify(body),
    });

    if (!response.ok) {
      const errorText = await response.text();
      throw new Error(`Anthropic API error (${response.status}): ${errorText}`);
    }

    const json = await response.json();
    const text = json.content?.[0]?.text?.trim() || '';
    const responseTimeMs = Date.now() - startTime;

    return { text, responseTimeMs };
  }
}
