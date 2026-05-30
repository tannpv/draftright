import { AiProvider } from '../entities/ai-provider.entity';

/**
 * The contract every provider-specific wire implementation satisfies.
 *
 * One file per upstream API (OpenAI-shape, Anthropic-shape, etc.). The
 * registry (`ProviderStrategyRegistry`) picks the right strategy by
 * calling `matches(provider)` on each — `AiProvidersService.callProvider`
 * never branches on `type` directly. Adding a fourth backend (Gemini,
 * Mistral, gRPC, …) becomes:
 *
 *   1. New file under `strategies/` implementing this interface.
 *   2. Register it in `AiProvidersModule.providers` AND in the
 *      `PROVIDER_STRATEGIES` array consumed by the registry.
 *   3. Zero edits to existing strategies or to the service layer.
 *
 * That's the open/closed half of clean architecture: extend by adding
 * a new strategy file; never modify the existing ones.
 */
export interface ProviderStrategy {
  /**
   * True when this strategy can handle the given provider config.
   *
   * Most strategies decide on `provider.type`. The OpenAI strategy
   * also covers OPENAI-compat servers (Ollama Cloud, vLLM, …)
   * because they speak the same /v1/chat/completions wire.
   */
  matches(provider: AiProvider): boolean;

  /**
   * Issue the request and return the rewritten text + the
   * server-measured round-trip duration in milliseconds.
   *
   * `startTime` is the millisecond timestamp the caller captured
   * BEFORE choosing a strategy — strategies must subtract from
   * Date.now() at success time so cross-strategy latency comparisons
   * stay apples-to-apples.
   *
   * Implementations are responsible for:
   *   - Building the wire payload (model, temperature, messages, …).
   *   - Applying provider-specific quirks (gpt-5 reasoning_effort,
   *     Anthropic system-prompt placement, …).
   *   - Retrying on transient upstream 429s.
   *   - Throwing `Error('AI provider error (<status>): <body>')`
   *     on non-2xx so the caller wraps it as a 502.
   */
  call(
    provider: AiProvider,
    systemPrompt: string,
    userText: string,
    startTime: number,
  ): Promise<{ text: string; responseTimeMs: number }>;
}
