import 'package:flutter_test/flutter_test.dart';
import 'package:draftright_mobile/models/language_module.dart';

void main() {
  group('LanguagePack.packFileId', () {
    test('strips .pack extension → matches JapanesePackResolver pattern', () {
      const pack = LanguagePack(
        url: 'https://draftright.info/ime-packs/draftright-ime-ja-v1.pack',
        version: 1, sizeBytes: 0, sha256: '',
      );
      expect(pack.packFileId, 'draftright-ime-ja-v1');
    });

    test('wordlist pack derives correct id', () {
      const pack = LanguagePack(
        url: 'https://draftright.info/ime-packs/draftright-wordlist-vi-v2.pack',
        version: 2, sizeBytes: 0, sha256: '',
      );
      expect(pack.packFileId, 'draftright-wordlist-vi-v2');
    });

    test('url without extension returns full filename', () {
      const pack = LanguagePack(
        url: 'https://example.com/packs/mypack',
        version: 1, sizeBytes: 0, sha256: '',
      );
      expect(pack.packFileId, 'mypack');
    });
  });
}
