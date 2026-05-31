import { Inject, Injectable } from '@nestjs/common';
import { AiProvider } from '../entities/ai-provider.entity';
import { ProviderStrategy } from './provider-strategy.interface';
import { OpenAiStrategy } from './openai.strategy';
import { AnthropicStrategy } from './anthropic.strategy';

/**
 * DI token under which the array of strategies is injected. Module
 * wires it with `{ provide: PROVIDER_STRATEGIES, useFactory: (a, b, ...) => [a, b, ...] }`.
 *
 * Symbol-based (not a string literal) to make accidental collisions
 * with another module that re-uses the same string impossible.
 * NestJS resolves both shapes; symbols are the canonical recommendation.
 */
export const PROVIDER_STRATEGIES = Symbol('PROVIDER_STRATEGIES');

/**
 * Routes a provider config to the first strategy that claims it.
 *
 * Order in the constructor-injected array matters only when two
 * strategies could match the same config — today that doesn't happen
 * (matches() are disjoint by type). When adding a fourth strategy,
 * keep the most-specific matcher first.
 */
@Injectable()
export class ProviderStrategyRegistry {
  constructor(
    @Inject(PROVIDER_STRATEGIES)
    private readonly strategies: ProviderStrategy[],
  ) {}

  /**
   * Picks the strategy for the given provider. Throws when no
   * strategy claims it — that's a configuration error worth surfacing
   * loudly rather than silently degrading to one of the others.
   */
  pick(provider: AiProvider): ProviderStrategy {
    const match = this.strategies.find(s => s.matches(provider));
    if (!match) {
      throw new Error(
        `No provider strategy registered for type "${provider.type}" (provider id=${provider.id}, name=${provider.name})`,
      );
    }
    return match;
  }
}

/**
 * Factory used by the module to bundle all available strategies into
 * the array under PROVIDER_STRATEGIES. Adding a new strategy = adding
 * one more parameter here AND one more dependency on the providers list
 * in the module file. Two edits, zero scattered conditionals.
 */
export function buildProviderStrategies(
  openai: OpenAiStrategy,
  anthropic: AnthropicStrategy,
): ProviderStrategy[] {
  return [openai, anthropic];
}
