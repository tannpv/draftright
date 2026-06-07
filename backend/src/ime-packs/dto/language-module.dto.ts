// The uniform descriptor for every keyboard language in the container model.
// A language can ship with two kinds of optional downloadable data:
//   * `pack`         — the input-engine data (RIME schemas + dict for JP/ZH/KO).
//                      Required when bundled=false.
//   * `wordlistPack` — the suggestion-engine data (trigram word + bigram lists
//                      for Latin scripts: VI, EN, FR, ES, DE, IT, PT). Optional
//                      even for bundled languages; the IME falls back to its
//                      built-in bootstrap list if the pack hasn't been
//                      installed yet.
//
// The manifest is the server-driven catalog: publishing a new pack makes
// suggestions appear in-app with no app update.

export type InputMethod = 'composition' | 'candidate' | 'passthrough';
export type EngineKind = 'composition' | 'rime' | 'dictionary' | 'none';

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
  bundled: boolean; // true => engine ships in the app, no download
  pack?: LanguagePack; // engine data (RIME); present iff !bundled
  wordlistPack?: LanguagePack; // suggestion data; optional for any language
}
