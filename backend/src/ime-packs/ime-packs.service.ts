import { Injectable } from '@nestjs/common';
import { LanguageModule } from './dto/language-module.dto';

const PACK_BASE = 'https://draftright.info/ime-packs';

/**
 * Server-driven catalog of keyboard languages (the "language container").
 * Bundled languages ship in the app; candidate languages carry a downloadable
 * RIME dictionary pack. Pack sha256/sizeBytes are filled at publish time by the
 * pack build/publish step (scripts/build-ime-pack.sh) — empty here until then.
 */
@Injectable()
export class ImePacksService {
  private readonly modules: LanguageModule[] = [
    {
      id: 'en', displayName: 'English', inputMethod: 'passthrough', engine: 'none', layout: 'qwerty', bundled: true,
      wordlistPack: { url: `${PACK_BASE}/draftright-wordlist-en-v1.tsv`, version: 1, sizeBytes: 0, sha256: '', minEngineVersion: 1 },
    },
    {
      id: 'vi', displayName: 'Tiếng Việt', inputMethod: 'composition', engine: 'composition', layout: 'qwerty', bundled: true,
      // First downloadable wordlist — replaces the in-APK ~200-entry bootstrap
      // once installed. sha256/sizeBytes filled at publish time by
      // scripts/build-word-list-pack.sh; empty here is the "pack not yet
      // built / not yet uploaded" state, which the client safely ignores.
      wordlistPack: { url: `${PACK_BASE}/draftright-wordlist-vi-v1.tsv`, version: 1, sizeBytes: 0, sha256: '', minEngineVersion: 1 },
    },
    {
      id: 'fr', displayName: 'Français', inputMethod: 'composition', engine: 'composition', layout: 'qwerty', bundled: true,
      wordlistPack: { url: `${PACK_BASE}/draftright-wordlist-fr-v1.tsv`, version: 1, sizeBytes: 0, sha256: '', minEngineVersion: 1 },
    },
    { id: 'es', displayName: 'Español', inputMethod: 'composition', engine: 'composition', layout: 'qwerty', bundled: true },
    { id: 'de', displayName: 'Deutsch', inputMethod: 'composition', engine: 'composition', layout: 'qwerty', bundled: true },
    { id: 'it', displayName: 'Italiano', inputMethod: 'composition', engine: 'composition', layout: 'qwerty', bundled: true },
    { id: 'pt', displayName: 'Português', inputMethod: 'composition', engine: 'composition', layout: 'qwerty', bundled: true },
    { id: 'ko', displayName: '한국어', inputMethod: 'composition', engine: 'composition', layout: 'qwerty', bundled: true },
    {
      id: 'ja',
      displayName: '日本語',
      inputMethod: 'candidate',
      engine: 'dictionary',   // JapaneseDictionaryEngine (pivot from librime)
      layout: 'romaji',
      bundled: false,
      pack: {
        url: `${PACK_BASE}/draftright-ime-ja-v3.pack`,
        version: 3,
        sizeBytes: 2016095,
        sha256: '100584d329fa2bbe67d9764ee802b7548a12af9ead01e5e50c599281eaf05282',
        minEngineVersion: 1,
      },
    },
    {
      id: 'zh-pinyin',
      displayName: '中文 (拼音)',
      inputMethod: 'candidate',
      engine: 'rime',
      layout: 'pinyin',
      bundled: false,
      pack: { url: `${PACK_BASE}/draftright-ime-zh-pinyin-v1.pack`, version: 1, sizeBytes: 0, sha256: '', minEngineVersion: 1 },
    },
  ];

  catalog(): LanguageModule[] {
    return this.modules;
  }
}
