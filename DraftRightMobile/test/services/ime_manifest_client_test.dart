// TC: IMEMANIFEST-001  parses {languages:[...]} into LanguageModule list
// TC: IMEMANIFEST-002  fetchDownloadable returns only packs (candidate langs)
import 'dart:convert';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:draftright_mobile/services/ime_manifest_client.dart';

const _body = '''
{"languages":[
  {"id":"en","displayName":"English","inputMethod":"passthrough","engine":"none","layout":"qwerty","bundled":true},
  {"id":"ja","displayName":"日本語","inputMethod":"candidate","engine":"rime","layout":"qwerty",
   "pack":{"url":"http://x/ja.pack","version":1,"sizeBytes":18874368,"sha256":"abc"}}
]}''';

void main() {
  test('parses the manifest into language modules', () async {
    final c = ImeManifestClient(
      baseUrl: 'http://h',
      httpClient: _mock((_) => (200, _body)),
    );
    final mods = await c.fetch();
    expect(mods.length, 2);
    expect(mods.first.id, 'en');
    expect(mods.first.requiresDownload, isFalse);
    final ja = mods.firstWhere((m) => m.id == 'ja');
    expect(ja.requiresDownload, isTrue);
    expect(ja.pack!.sizeLabel, '≈18 MB');
  });

  test('fetchDownloadable returns only candidate languages', () async {
    final c = ImeManifestClient(
      baseUrl: 'http://h',
      httpClient: _mock((_) => (200, _body)),
    );
    final dl = await c.fetchDownloadable();
    expect(dl.map((m) => m.id), ['ja']);
  });

  test('throws on a non-200 response', () async {
    final c = ImeManifestClient(
      baseUrl: 'http://h',
      httpClient: _mock((_) => (500, 'nope')),
    );
    await expectLater(c.fetch(), throwsA(isA<Exception>()));
  });
}

http.Client _mock((int, String) Function(http.Request) handler) =>
    _MockClient(handler);

class _MockClient extends http.BaseClient {
  final (int, String) Function(http.Request) handler;
  _MockClient(this.handler);
  @override
  Future<http.StreamedResponse> send(http.BaseRequest request) async {
    final (code, body) = handler(request as http.Request);
    return http.StreamedResponse(Stream.value(utf8.encode(body)), code,
        request: request);
  }
}
