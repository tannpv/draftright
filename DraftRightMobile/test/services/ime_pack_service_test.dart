// TC: IMEPACK-001  download + verify sha256 + atomic install
// TC: IMEPACK-002  reject hash mismatch (no leftover)
// TC: IMEPACK-003  remove an installed pack
// TC: IMEPACK-004  report download progress
import 'dart:convert';
import 'dart:io';
import 'package:crypto/crypto.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:draftright_mobile/services/ime_pack_service.dart';

void main() {
  late Directory tmp;
  setUp(() => tmp = Directory.systemTemp.createTempSync('imepack_test'));
  tearDown(() {
    if (tmp.existsSync()) tmp.deleteSync(recursive: true);
  });

  // A deterministic fake pack body + its real sha256.
  final bytes = utf8.encode('FAKE-JA-RIME-PACK-DATA-' * 500);
  final goodHash = sha256.convert(bytes).toString();

  http.Client clientFor(List<int> body, {int code = 200}) =>
      _MockClient((req) async => http.StreamedResponse(
            Stream.fromIterable([body.sublist(0, body.length ~/ 2), body.sublist(body.length ~/ 2)]),
            code,
            contentLength: body.length,
            request: req,
          ));

  test('downloads, verifies sha256, installs atomically', () async {
    final svc = ImePackService(baseDir: tmp, httpClient: clientFor(bytes));
    final path = await svc.install(
        packId: 'ja', url: 'http://x/ja.pack', sha256: goodHash, sizeBytes: bytes.length);
    expect(File(path).existsSync(), isTrue);
    expect(File(path).readAsBytesSync(), bytes);
    expect(await svc.isInstalled('ja'), isTrue);
    expect(File('$path.part').existsSync(), isFalse, reason: 'temp file cleaned up');
  });

  test('rejects a pack whose hash mismatches and leaves nothing behind', () async {
    final svc = ImePackService(baseDir: tmp, httpClient: clientFor(bytes));
    await expectLater(
      svc.install(packId: 'ja', url: 'http://x/ja.pack', sha256: 'deadbeef', sizeBytes: bytes.length),
      throwsA(isA<PackIntegrityError>()),
    );
    expect(await svc.isInstalled('ja'), isFalse);
    expect(File('${svc.packPath('ja')}.part').existsSync(), isFalse);
  });

  test('remove deletes an installed pack', () async {
    final svc = ImePackService(baseDir: tmp, httpClient: clientFor(bytes));
    await svc.install(packId: 'ja', url: 'u', sha256: goodHash, sizeBytes: bytes.length);
    await svc.remove('ja');
    expect(await svc.isInstalled('ja'), isFalse);
  });

  test('reports download progress ending at 1.0', () async {
    final svc = ImePackService(baseDir: tmp, httpClient: clientFor(bytes));
    final seen = <double>[];
    await svc.install(
        packId: 'ja', url: 'u', sha256: goodHash, sizeBytes: bytes.length, onProgress: seen.add);
    expect(seen, isNotEmpty);
    expect(seen.last, closeTo(1.0, 0.0001));
  });

  test('throws on a non-200 response', () async {
    final svc = ImePackService(baseDir: tmp, httpClient: clientFor(bytes, code: 404));
    await expectLater(
      svc.install(packId: 'ja', url: 'u', sha256: goodHash, sizeBytes: bytes.length),
      throwsA(isA<Exception>()),
    );
  });
}

class _MockClient extends http.BaseClient {
  final Future<http.StreamedResponse> Function(http.BaseRequest) handler;
  _MockClient(this.handler);

  @override
  Future<http.StreamedResponse> send(http.BaseRequest request) => handler(request);
}
