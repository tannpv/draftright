/**
 * Tuning constants + the single prompt-injection builder for per-user context
 * (#173). One source of truth: the entity column widths, the DTO validators,
 * and the injected preamble all read these — never a literal at a call site.
 */

/** Max length of each short profile field (job title, industry, audience). */
export const PROFILE_FIELD_MAX_LEN = 120;

/** Max length of the free-text style notes the user writes. */
export const STYLE_NOTES_MAX_LEN = 1000;

/**
 * Hard cap on the injected context block, so a long profile can never blow the
 * prompt's token budget. Style notes are truncated to fit; the structured
 * fields are already length-limited above.
 */
export const CONTEXT_PREAMBLE_MAX_CHARS = 900;

/** The shape the builder needs — the persisted, decrypted profile. */
export interface UserContextProfile {
  enabled: boolean;
  job_title: string;
  industry: string;
  audience: string;
  style_notes: string;
}

function truncate(value: string, max: number): string {
  // Reserve one char for the ellipsis so the result is never longer than max.
  return value.length <= max ? value : value.slice(0, max - 1).trimEnd() + '…';
}

/**
 * Build the system-context preamble prepended to the tone prompt at rewrite
 * time, or `null` when there is nothing to inject (disabled, or every field
 * empty) — a null result means the rewrite behaves exactly as it does today,
 * at zero cost.
 *
 * Pure and dependency-free so it is unit-testable and so the Go rewrite path
 * can mirror it byte-for-byte (the prod `/rewrite` runs on Go — this string
 * must match what Go produces, or the two backends personalize differently).
 * Instruct the model NOT to mention the context so it never leaks into output.
 */
export function buildContextPreamble(ctx: UserContextProfile): string | null {
  if (!ctx.enabled) return null;

  const who: string[] = [];
  if (ctx.job_title) who.push(ctx.job_title);
  if (ctx.industry) who.push(`in ${ctx.industry}`);
  if (ctx.audience) who.push(`writing for ${ctx.audience}`);

  const style = ctx.style_notes.trim();
  if (who.length === 0 && !style) return null;

  const parts: string[] = [];
  if (who.length > 0) parts.push(`About the person you are writing for: ${who.join(', ')}.`);
  if (style) parts.push(`Their writing style: ${style}.`);
  parts.push('Apply this to the rewrite, but do not mention it or address the person.');

  const block = parts.join(' ');
  return truncate(block, CONTEXT_PREAMBLE_MAX_CHARS) + '\n\n';
}
