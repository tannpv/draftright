import {
  ProviderStrategyRegistry,
  buildProviderStrategies,
} from './provider-strategy.registry';
import { AiProvider, AiProviderType } from '../entities/ai-provider.entity';
import { ProviderStrategy } from './provider-strategy.interface';

function makeProvider(type: AiProviderType, overrides: Partial<AiProvider> = {}): AiProvider {
  return {
    id: 'id-123',
    name: 'test',
    type,
    endpoint_url: 'https://example.invalid/v1/chat/completions',
    api_key: 'k',
    model: 'm',
    temperature: 0.3,
    is_default: true,
    is_active: true,
    created_at: new Date(),
    updated_at: new Date(),
    ...overrides,
  } as AiProvider;
}

class FakeStrategy implements ProviderStrategy {
  constructor(private readonly type: AiProviderType, public label: string) {}
  matches(p: AiProvider) { return p.type === this.type; }
  async call() { return { text: this.label, responseTimeMs: 1 }; }
}

describe('ProviderStrategyRegistry', () => {
  it('picks the strategy whose matches() returns true', () => {
    const openai = new FakeStrategy(AiProviderType.OPENAI, 'openai-result');
    const anthropic = new FakeStrategy(AiProviderType.ANTHROPIC, 'anthropic-result');
    const registry = new ProviderStrategyRegistry([openai, anthropic]);

    expect(registry.pick(makeProvider(AiProviderType.OPENAI))).toBe(openai);
    expect(registry.pick(makeProvider(AiProviderType.ANTHROPIC))).toBe(anthropic);
  });

  it('returns the FIRST matching strategy when multiple claim a provider', () => {
    const first = new FakeStrategy(AiProviderType.OPENAI, 'first');
    const second = new FakeStrategy(AiProviderType.OPENAI, 'second');
    const registry = new ProviderStrategyRegistry([first, second]);
    expect(registry.pick(makeProvider(AiProviderType.OPENAI))).toBe(first);
  });

  it('throws when no strategy matches', () => {
    const registry = new ProviderStrategyRegistry([
      new FakeStrategy(AiProviderType.OPENAI, 'a'),
    ]);
    expect(() => registry.pick(makeProvider(AiProviderType.ANTHROPIC)))
      .toThrow(/No provider strategy registered for type "anthropic"/);
  });

  it('buildProviderStrategies bundles the args into an array preserving order', () => {
    const openai = new FakeStrategy(AiProviderType.OPENAI, 'o');
    const anthropic = new FakeStrategy(AiProviderType.ANTHROPIC, 'a');
    const bundled = buildProviderStrategies(openai as any, anthropic as any);
    expect(bundled).toEqual([openai, anthropic]);
  });
});
