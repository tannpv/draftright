// The uniform descriptor for every keyboard language in the container model.
// Bundled languages (.composition / .passthrough) ship in the app with no data
// download; candidate languages (.candidate, RIME-driven) carry a downloadable
// dictionary pack. The manifest is the server-driven catalog: publishing a new
// pack makes a language appear in-app with no app update.

export type InputMethod = 'composition' | 'candidate' | 'passthrough';
export type EngineKind = 'composition' | 'rime' | 'none';

export interface LanguagePack {
  url: string;
  version: number;
  sizeBytes: number;
  sha256: string;
  /** Bundled engine must be at least this version to load the pack. */
  minEngineVersion: number;
}

export interface LanguageModule {
  id: string; // BCP-47-ish: "ja", "zh-pinyin", "ko", "vi", "en"
  displayName: string;
  inputMethod: InputMethod;
  engine: EngineKind;
  layout: string; // "qwerty" | "romaji" | "pinyin"
  bundled: boolean; // true => ships in the app, no download
  pack?: LanguagePack; // present iff !bundled
}
