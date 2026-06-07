import { ImePacksService } from './ime-packs.service';

describe('ImePacksService — language container catalog', () => {
  const svc = new ImePacksService();

  it('lists Japanese as a downloadable candidate (rime) pack', () => {
    const ja = svc.catalog().find((m) => m.id === 'ja');
    expect(ja).toBeDefined();
    expect(ja!.inputMethod).toBe('candidate');
    expect(ja!.engine).toBe('rime');
    expect(ja!.bundled).toBe(false);
    expect(ja!.pack).toMatchObject({
      url: expect.stringContaining('.pack'),
      version: expect.any(Number),
      minEngineVersion: expect.any(Number),
    });
  });

  it('lists Korean as a bundled composition language (no pack)', () => {
    const ko = svc.catalog().find((m) => m.id === 'ko');
    expect(ko).toBeDefined();
    expect(ko!.inputMethod).toBe('composition');
    expect(ko!.bundled).toBe(true);
    expect(ko!.pack).toBeUndefined();
  });

  it('every module has the uniform descriptor shape', () => {
    for (const m of svc.catalog()) {
      expect(typeof m.id).toBe('string');
      expect(typeof m.displayName).toBe('string');
      expect(['composition', 'candidate', 'passthrough']).toContain(m.inputMethod);
      expect(['composition', 'rime', 'none']).toContain(m.engine);
      expect(typeof m.bundled).toBe('boolean');
      // candidate languages must carry a downloadable pack; bundled ones must not
      if (m.inputMethod === 'candidate') expect(m.bundled).toBe(false);
      if (!m.bundled) expect(m.pack).toBeDefined();
      if (m.bundled) expect(m.pack).toBeUndefined();
    }
  });
});
