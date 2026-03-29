import { Injectable, HttpException } from '@nestjs/common';
import { SubscriptionsService } from '../subscriptions/subscriptions.service';
import { UsageService } from '../usage/usage.service';
import { AiProvidersService } from '../ai-providers/ai-providers.service';

const TONE_PROMPTS: Record<string, string> = {
  simple: 'Rewrite the following text using simple, easy-to-understand language. Use short sentences and common words. Preserve the original meaning. Return only the rewritten text, no explanations.',
  natural: 'Rewrite the following text to sound more natural and conversational, as if spoken by a real person. Remove awkward phrasing and make it flow smoothly. Preserve the original meaning. Return only the rewritten text, no explanations.',
  polished: 'Rewrite the following text to be more polished and professional. Improve grammar, word choice, and sentence structure for a refined, workplace-appropriate tone. Preserve the original meaning. Return only the rewritten text, no explanations.',
  concise: 'Rewrite the following text to be as concise as possible. Remove unnecessary words, redundancy, and filler while preserving the key meaning. Return only the rewritten text, no explanations.',
  technical: 'Rewrite the following text in a technical specification style. Use precise, unambiguous language suitable for documentation, specs, or technical communication. Preserve the original meaning. Return only the rewritten text, no explanations.',
};

function getTranslatePrompt(targetLanguage: string): string {
  return `Translate the following text into ${targetLanguage}. If the text is already in ${targetLanguage}, translate it into English instead. Preserve the original meaning and tone. Return only the translated text, no explanations.`;
}

@Injectable()
export class RewriteService {
  constructor(
    private readonly subscriptionsService: SubscriptionsService,
    private readonly usageService: UsageService,
    private readonly aiProvidersService: AiProvidersService,
  ) {}

  async rewrite(userId: string, text: string, tone: string, targetLanguage?: string) {
    const sub = await this.subscriptionsService.findActiveByUserId(userId);
    if (!sub || !sub.plan) {
      throw new HttpException({ error: 'No active subscription', usage_today: 0, daily_limit: 0 }, 403);
    }

    const dailyLimit = sub.plan.daily_limit;
    const usageToday = await this.usageService.countTodayByUser(userId);

    if (dailyLimit !== -1 && usageToday >= dailyLimit) {
      throw new HttpException({
        error: 'Daily limit reached', usage_today: usageToday, daily_limit: dailyLimit,
      }, 429);
    }

    const systemPrompt = tone === 'translate'
      ? getTranslatePrompt(targetLanguage || 'English')
      : TONE_PROMPTS[tone];

    if (!systemPrompt) {
      throw new HttpException({ error: `Unknown tone: ${tone}` }, 400);
    }

    const provider = await this.aiProvidersService.findDefault();

    let result: { text: string; responseTimeMs: number };
    try {
      result = await this.aiProvidersService.callProvider(provider, systemPrompt, text);
    } catch (error: any) {
      throw new HttpException({ error: `AI provider error: ${error.message}` }, 502);
    }

    await this.usageService.log({
      user_id: userId, tone, input_length: text.length, output_length: result.text.length,
      ai_provider_id: provider.id, response_time_ms: result.responseTimeMs,
    });

    return { rewritten_text: result.text, usage_today: usageToday + 1, daily_limit: dailyLimit };
  }
}
