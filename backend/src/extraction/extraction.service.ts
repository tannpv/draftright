import { Injectable, Logger } from '@nestjs/common';
import { AiProvidersService } from '../ai-providers/ai-providers.service';
import {
  EntityKind,
  ExtractResponseDto,
  ExtractedEntityDto,
} from './dto/extract.dto';

const REGEX_HANDLED = new Set<EntityKind>([
  EntityKind.Phone,
  EntityKind.Email,
  EntityKind.Url,
  EntityKind.Otp,
  EntityKind.CreditCard,
]);

@Injectable()
export class ExtractionService {
  private readonly logger = new Logger(ExtractionService.name);

  constructor(private readonly aiProviders: AiProvidersService) {}

  async extract(
    text: string,
    kinds?: EntityKind[],
  ): Promise<ExtractResponseDto> {
    const provider = await this.aiProviders.findDefault();
    const system = this.buildSystemPrompt(kinds);
    const user = text;
    const { text: rawOutput, responseTimeMs } =
      await this.aiProviders.callProvider(provider, system, user);

    let parsed: any;
    try {
      const cleaned = this.stripCodeFences(rawOutput);
      parsed = JSON.parse(cleaned);
    } catch (e) {
      this.logger.warn(`extraction_llm_unparseable: ${rawOutput.slice(0, 200)}`);
      return { entities: [], provider: provider.name, tokensUsed: 0 };
    }
    if (!Array.isArray(parsed)) {
      return { entities: [], provider: provider.name, tokensUsed: 0 };
    }

    const out: ExtractedEntityDto[] = [];
    for (const raw of parsed) {
      const v = this.validateEntity(raw, text);
      if (v) out.push(v);
    }
    return {
      entities: this.dedupe(out),
      provider: provider.name,
      tokensUsed: this.estimateTokens(text + rawOutput),
    };
  }

  private buildSystemPrompt(kinds?: EntityKind[]): string {
    const allowed = (kinds ?? [
      EntityKind.Address,
      EntityKind.PersonName,
      EntityKind.DateTime,
      EntityKind.BankAccount,
    ]).filter((k) => !REGEX_HANDLED.has(k));
    return [
      'You extract structured entities from short messages.',
      'Return strict JSON array, no commentary. No code fences.',
      'Each item: {kind, value, display, confidence, meta?}.',
      `Kinds you MAY emit: ${allowed.join('|')}.`,
      `Kinds you MUST NOT emit (handled by client regex): ${[...REGEX_HANDLED].join('|')}.`,
      'value MUST be a literal substring of the input. confidence is 0..1.',
      'Example input: "Địa chỉ 123 Lê Lợi, Q1. Vietcombank 0123456789"',
      'Example output: [{"kind":"address","value":"123 Lê Lợi, Q1","display":"123 Lê Lợi, Q1","confidence":0.9},{"kind":"bankAccount","value":"0123456789","display":"Vietcombank · 0123456789","confidence":0.95,"meta":{"bank":"Vietcombank"}}]',
    ].join('\n');
  }

  private stripCodeFences(s: string): string {
    const trimmed = s.trim();
    if (trimmed.startsWith('```')) {
      const firstNl = trimmed.indexOf('\n');
      const body = firstNl < 0 ? '' : trimmed.slice(firstNl + 1);
      const endIdx = body.lastIndexOf('```');
      return endIdx >= 0 ? body.slice(0, endIdx).trim() : body.trim();
    }
    return trimmed;
  }

  private validateEntity(raw: any, text: string): ExtractedEntityDto | null {
    if (typeof raw !== 'object' || raw === null) return null;
    const kind = raw.kind;
    if (!Object.values(EntityKind).includes(kind)) return null;
    if (REGEX_HANDLED.has(kind)) return null;          // defense in depth
    const value = typeof raw.value === 'string' ? raw.value : null;
    if (!value) return null;
    const start = text.indexOf(value);
    if (start < 0) {
      this.logger.warn(`extraction_hallucination: ${kind}=${value}`);
      return null;
    }
    const display =
      typeof raw.display === 'string' && raw.display.trim() ? raw.display : value;
    const confidenceRaw = typeof raw.confidence === 'number' ? raw.confidence : 0.5;
    const confidence = Math.max(0, Math.min(1, confidenceRaw));
    const meta =
      raw.meta && typeof raw.meta === 'object'
        ? Object.fromEntries(
            Object.entries(raw.meta).map(([k, v]) => [String(k), String(v)]),
          )
        : undefined;
    return {
      kind,
      value,
      display,
      start,
      end: start + value.length,
      confidence,
      meta,
    };
  }

  private dedupe(items: ExtractedEntityDto[]): ExtractedEntityDto[] {
    const byKey = new Map<string, ExtractedEntityDto>();
    for (const e of items) {
      const key = `${e.kind}:${e.value.toLowerCase()}`;
      const cur = byKey.get(key);
      if (!cur || e.confidence > cur.confidence) byKey.set(key, e);
    }
    return [...byKey.values()].sort((a, b) => a.start - b.start);
  }

  private estimateTokens(s: string): number {
    return Math.ceil(s.length / 4);   // rough heuristic; replace later
  }
}
