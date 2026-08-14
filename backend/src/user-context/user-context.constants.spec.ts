import {
  buildContextPreamble,
  STYLE_NOTES_INJECTED_MAX_CHARS,
  UserContextProfile,
} from './user-context.constants';

const base: UserContextProfile = {
  enabled: true,
  job_title: '',
  industry: '',
  audience: '',
  style_notes: '',
};

describe('buildContextPreamble', () => {
  it('returns null when disabled, even with a full profile', () => {
    expect(
      buildContextPreamble({ ...base, enabled: false, job_title: 'Lawyer', style_notes: 'formal' }),
    ).toBeNull();
  });

  it('returns null when enabled but every field empty (zero-cost no-op)', () => {
    expect(buildContextPreamble(base)).toBeNull();
  });

  it('builds a who-clause from job/industry/audience', () => {
    const out = buildContextPreamble({
      ...base,
      job_title: 'Lawyer',
      industry: 'finance',
      audience: 'clients',
    });
    expect(out).toContain('About the person you are writing for: Lawyer, in finance, writing for clients.');
  });

  it('includes style notes when present', () => {
    const out = buildContextPreamble({ ...base, style_notes: 'British spelling, no emojis' });
    expect(out).toContain('Their writing style: British spelling, no emojis.');
  });

  it('always instructs the model not to mention the context', () => {
    const out = buildContextPreamble({ ...base, job_title: 'Engineer' });
    expect(out).toContain('do not mention it');
  });

  it('works with style notes only (no structured fields)', () => {
    const out = buildContextPreamble({ ...base, style_notes: 'concise' });
    expect(out).not.toBeNull();
    expect(out).toContain('Their writing style: concise.');
  });

  it('ends with a blank-line separator so it prepends cleanly to the tone prompt', () => {
    const out = buildContextPreamble({ ...base, job_title: 'Nurse' })!;
    expect(out.endsWith('\n\n')).toBe(true);
  });

  it('caps the injected notes but ALWAYS keeps the no-mention instruction (long profile)', () => {
    const out = buildContextPreamble({ ...base, style_notes: 'x'.repeat(5000) })!;
    // notes are truncated to the field budget…
    expect(out).toContain('…');
    // …and the safety instruction is never truncated away (the bug being fixed)
    expect(out).toContain('do not mention it');
    expect(out.endsWith('\n\n')).toBe(true);
    // injected notes never exceed the field budget
    const notes = out.split('Their writing style: ')[1].split('.')[0];
    expect(notes.length).toBeLessThanOrEqual(STYLE_NOTES_INJECTED_MAX_CHARS);
  });
});
