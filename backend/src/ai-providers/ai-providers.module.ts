import { Module } from '@nestjs/common';
import { TypeOrmModule } from '@nestjs/typeorm';
import { AiProvider } from './entities/ai-provider.entity';
import { AiProvidersService } from './ai-providers.service';
import { OpenAiStrategy } from './strategies/openai.strategy';
import { AnthropicStrategy } from './strategies/anthropic.strategy';
import {
  ProviderStrategyRegistry,
  PROVIDER_STRATEGIES,
  buildProviderStrategies,
} from './strategies/provider-strategy.registry';

/**
 * AiProvidersModule wires the strategy pattern that replaced the
 * branchy callProvider() (architecture review item 3, 2026-05-30).
 *
 * Provider list:
 *   - OpenAiStrategy / AnthropicStrategy: concrete wire impls.
 *   - PROVIDER_STRATEGIES: array DI token holding [openai, anthropic],
 *     consumed by ProviderStrategyRegistry.
 *   - ProviderStrategyRegistry: pick(provider) → matching strategy.
 *   - AiProvidersService: CRUD + thin dispatch via the registry.
 *
 * Adding a fourth backend (e.g. Gemini):
 *   1. New file src/ai-providers/strategies/gemini.strategy.ts.
 *   2. Add GeminiStrategy here as a provider.
 *   3. Extend buildProviderStrategies() signature + body to include
 *      it. Update the factory `inject` array to match.
 * Three local edits. No callers change.
 */
@Module({
  imports: [TypeOrmModule.forFeature([AiProvider])],
  providers: [
    AiProvidersService,
    OpenAiStrategy,
    AnthropicStrategy,
    ProviderStrategyRegistry,
    {
      provide: PROVIDER_STRATEGIES,
      useFactory: buildProviderStrategies,
      inject: [OpenAiStrategy, AnthropicStrategy],
    },
  ],
  exports: [AiProvidersService],
})
export class AiProvidersModule {}
