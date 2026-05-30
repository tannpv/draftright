import { Injectable, Logger } from '@nestjs/common';
import { AiProvider, AiProviderType } from '../entities/ai-provider.entity';
import { ProviderStrategy } from './provider-strategy.interface';

/**
 * OpenAI-compatible wire: POST /v1/chat/completions with a list of
 * messages and either a temperature OR — for gpt-5 reasoning models —
 * a `reasoning_effort` knob plus the omission of temperature.
 *
 * Covers four real-world server families:
 *   - OpenAI proper          (gpt-4o-mini, gpt-5-nano, …)
 *   - OpenAI-compat servers  (Ollama Cloud, vLLM, Together, …)
 *   - Local Ollama           (when AiProviderType.OLLAMA flips on)
 *   - Custom (`type=custom`) self-hosted OpenAI-shape endpoints
 *
 * All four are dispatched here because the wire format is identical;
 * the `isLocal` branch only affects the system-vs-user prompt layout,
 * not the HTTP shape.
 */
@Injectable()
export class OpenAiStrategy implements ProviderStrategy {
  private static readonly MAX_429_RETRIES = 3;
  private static readonly RETRY_BACKOFF_MS = 400;
  private readonly logger = new Logger(OpenAiStrategy.name);

  matches(provider: AiProvider): boolean {
    return (
      provider.type === AiProviderType.OPENAI ||
      provider.type === AiProviderType.OLLAMA ||
      provider.type === AiProviderType.CUSTOM
    );
  }

  async call(
    provider: AiProvider,
    systemPrompt: string,
    userText: string,
    startTime: number,
    attempt = 1,
  ): Promise<{ text: string; responseTimeMs: number }> {
    const body = this.buildBody(provider, systemPrompt, userText);

    const headers: Record<string, string> = { 'Content-Type': 'application/json' };
    if (provider.api_key) {
      headers['Authorization'] = `Bearer ${provider.api_key}`;
    }

    const response = await fetch(provider.endpoint_url, {
      method: 'POST',
      headers,
      body: JSON.stringify(body),
    });

    // Transient-load retry. Ollama Cloud free tier + OpenAI burst limits
    // both surface as 429 'too many concurrent requests'. Short
    // exponential backoff masks bursts without forever hangs.
    if (response.status === 429 && attempt < OpenAiStrategy.MAX_429_RETRIES) {
      const wait = OpenAiStrategy.RETRY_BACKOFF_MS * attempt;
      this.logger.warn(`upstream 429, retrying in ${wait}ms (attempt ${attempt})`);
      await new Promise(resolve => setTimeout(resolve, wait));
      return this.call(provider, systemPrompt, userText, startTime, attempt + 1);
    }

    if (!response.ok) {
      const errorText = await response.text();
      throw new Error(`AI provider error (${response.status}): ${errorText}`);
    }

    const json: any = await response.json();
    const text: string = json?.choices?.[0]?.message?.content?.trim() ?? '';
    return { text, responseTimeMs: Date.now() - startTime };
  }

  /**
   * Assembles the wire body with provider-specific quirks centralised
   * here — gpt-5 family, local-model "instructions in user message"
   * convention, and the standard system+user messages otherwise.
   * Keeping ALL quirks in one method (and one strategy) means future
   * additions (e.g. tool_calls, json_mode) extend the body builder
   * without scattering across the codebase.
   */
  private buildBody(provider: AiProvider, systemPrompt: string, userText: string): Record<string, unknown> {
    const isLocal = provider.type === AiProviderType.OLLAMA;
    const isGpt5 = typeof provider.model === 'string' && provider.model.startsWith('gpt-5');

    const messages = isLocal
      ? [
          {
            role: 'system',
            content:
              'You are a writing assistant. If the user provides existing text, rewrite it as instructed. If the user provides a request or instruction, generate the requested content. You ONLY output the result text. Never answer questions about yourself, never explain your process, never add commentary. CRITICAL: You must reply in the SAME LANGUAGE as the input text. If the input is Vietnamese, reply in Vietnamese. If French, reply in French. Never switch to English unless the input is in English.',
          },
          {
            role: 'user',
            content: `${systemPrompt}\n\nIMPORTANT: Your response MUST be in the same language as the text below.\n\nUser input:\n"${userText}"\n\nOutput:`,
          },
        ]
      : [
          { role: 'system', content: systemPrompt },
          { role: 'user', content: userText },
        ];

    const body: Record<string, unknown> = { model: provider.model, messages };

    if (isGpt5) {
      // gpt-5 family: temperature MUST be omitted (only default
      // value 1 is supported); reasoning_effort=minimal disables
      // the costly hidden reasoning tokens for shallow rewrites.
      body.reasoning_effort = 'minimal';
    } else {
      body.temperature = Number(provider.temperature);
    }
    return body;
  }
}
