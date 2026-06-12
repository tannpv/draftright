import { TONES, TONE_IDS } from './tones';

describe('tone catalog', () => {
  it('has unique ids and TONE_IDS mirrors them in order', () => {
    const ids = TONES.map((t) => t.id);
    expect(new Set(ids).size).toBe(ids.length);
    expect(TONE_IDS).toEqual(ids);
  });

  it('every entry has a label, icon, and a valid kind', () => {
    const kinds = new Set(['rewrite', 'grammar', 'translate']);
    for (const t of TONES) {
      expect(t.label.length).toBeGreaterThan(0);
      expect(t.icon.length).toBeGreaterThan(0);
      expect(kinds.has(t.kind)).toBe(true);
    }
  });

  it('includes the grammar_check and translate special tones', () => {
    expect(TONES.find((t) => t.id === 'grammar_check')?.kind).toBe('grammar');
    expect(TONES.find((t) => t.id === 'translate')?.kind).toBe('translate');
  });
});
