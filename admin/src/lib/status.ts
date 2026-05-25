/** Shared status-badge styling. Pages map their own domain status → a Tone;
 * the color/bg pairs (built from the theme tokens) live here only. */
export type Tone = 'primary' | 'success' | 'warning' | 'danger' | 'info' | 'muted';

const STYLES: Record<Tone, { color: string; bg: string }> = {
  primary: { color: 'var(--primary)',   bg: 'rgba(93,135,255,0.12)' },
  success: { color: 'var(--success)',   bg: 'rgba(19,222,185,0.12)' },
  warning: { color: 'var(--warning)',   bg: 'rgba(255,174,31,0.12)' },
  danger:  { color: 'var(--danger)',    bg: 'rgba(250,137,107,0.12)' },
  info:    { color: 'var(--secondary)', bg: 'rgba(73,190,255,0.12)' },
  muted:   { color: 'var(--muted)',     bg: 'rgba(124,143,172,0.12)' },
};

export const toneStyle = (tone: Tone) => STYLES[tone];
