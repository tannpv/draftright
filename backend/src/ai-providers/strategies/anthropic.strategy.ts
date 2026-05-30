import { Injectable, Logger } from '@nestjs/common';
import { AiProvider, AiProviderType } from '../entities/ai-provider.entity';
import { ProviderStrategy } from './provider-strategy.interface';

/**
 * Anthropic Messages API — different wire from OpenAI:
 *   - x-api-key header + anthropic-version header
 *   - top-level `system` field (not in messages array)
 *   - `max_tokens` is REQUIRED
 *   - response is `content[0].text` instead of `choices[0].message.content`
 */
@Injectable()
export class AnthropicStrategy implements ProviderStrategy {
  private static readonly MAX_429_RETRIES = 3;
  private static readonly RETRY_BACKOFF_MS = 400;
  private static readonly API_VERSION = '2023-06-01';
  private static readonly DEFAULT_MAX_TOKENS = 1024;
  private readonly logger = new Logger(AnthropicStrategy.name);

  matches(provider: AiProvider): boolean {
    return provider.type === AiProviderType.ANTHROPIC;
  }

  async call(
    provider: AiProvider,
    systemPrompt: string,
    userText: string,
    startTime: number,
    attempt = 1,
  ): Promise<{ text: string; responseTimeMs: number }> {
    const body = {
      model: provider.model,
      max_tokens: AnthropicStrategy.DEFAULT_MAX_TOKENS,
      system: systemPrompt,
      messages: [{ role: 'user', content: userText }],
    };

    const response = await fetch(provider.endpoint_url, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        'x-api-key': provider.api_key,
        'anthropic-version': AnthropicStrategy.API_VERSION,
      },
      body: JSON.stringify(body),
    });

    if (response.status === 429 && attempt < AnthropicStrategy.MAX_429_RETRIES) {
      const wait = AnthropicStrategy.RETRY_BACKOFF_MS * attempt;
      this.logger.warn(`upstream 429, retrying in ${wait}ms (attempt ${attempt})`);
      await new Promise(resolve => setTimeout(resolve, wait));
      return this.call(provider, systemPrompt, userText, startTime, attempt + 1);
    }

    if (!response.ok) {
      const errorText = await response.text();
      throw new Error(`Anthropic API error (${response.status}): ${errorText}`);
    }

    const json: any = await response.json();
    const text: string = json?.content?.[0]?.text?.trim() ?? '';
    return { text, responseTimeMs: Date.now() - startTime };
  }
}
