/**
 * Canonical tone catalog — the single source of truth for the set of tones
 * the rewrite engine supports. Both the rewrite DTO validation and the public
 * `GET /rewrite/tones` endpoint derive from this list, and clients should
 * fetch that endpoint rather than hardcoding their own copy (which is how the
 * macOS / Flutter / keyboard / website lists drifted out of sync).
 *
 * `kind` tells a client how to treat the response:
 *   - rewrite:   returns { rewritten_text }
 *   - grammar:   returns { grammar: { score, issues[] } }
 *   - translate: returns { rewritten_text }, requires a target language
 */
export type ToneKind = 'rewrite' | 'grammar' | 'translate';

export interface ToneMeta {
  id: string;
  label: string;
  icon: string;
  kind: ToneKind;
}

export const TONES: ToneMeta[] = [
  { id: 'simple', label: 'Simple', icon: '✎', kind: 'rewrite' },
  { id: 'natural', label: 'Natural', icon: '💬', kind: 'rewrite' },
  { id: 'polished', label: 'Polished', icon: '✨', kind: 'rewrite' },
  { id: 'concise', label: 'Concise', icon: '⊖', kind: 'rewrite' },
  { id: 'technical', label: 'Technical', icon: '🔧', kind: 'rewrite' },
  { id: 'claude', label: 'Claude', icon: '✦', kind: 'rewrite' },
  { id: 'grammar_check', label: 'Grammar Check', icon: '✓', kind: 'grammar' },
  { id: 'translate', label: 'Translate', icon: '🌐', kind: 'translate' },
];

/** All valid tone ids, derived from the catalog (used for DTO validation). */
export const TONE_IDS: string[] = TONES.map((t) => t.id);

/** Valid input_kind values for POST /rewrite (used for DTO validation). */
export const INPUT_KIND_IDS = ['typed', 'speech'] as const;
export type InputKind = (typeof INPUT_KIND_IDS)[number];

/** Prepended to the tone prompt when the client marks input as dictated speech. */
export const SPEECH_PREAMBLE =
  'The input is dictated speech: remove filler words and false starts, restore punctuation and casing, keep the meaning and language. ';
