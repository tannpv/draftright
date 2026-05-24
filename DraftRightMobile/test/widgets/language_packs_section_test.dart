// TC: IMEUI-001  candidate language with no pack shows a Download affordance
// TC: IMEUI-002  tapping Download installs the pack, then shows Remove
// TC: IMEUI-003  an installed pack shows Remove and can be removed
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:draftright_mobile/models/language_module.dart';
import 'package:draftright_mobile/services/ime_pack_service.dart';
import 'package:draftright_mobile/widgets/language_packs_section.dart';

final _ja = const LanguageModule(
  id: 'ja',
  displayName: '日本語',
  inputMethod: 'candidate',
  engine: 'rime',
  layout: 'qwerty',
  pack: LanguagePack(
    url: 'http://example/ja.pack',
    version: 1,
    sizeBytes: 18 * 1024 * 1024,
    sha256: 'abc123',
  ),
);

Widget _wrap(PackInstaller svc, {bool installed = false}) => MaterialApp(
      home: Scaffold(
        body: LanguagePacksSection(modules: [_ja], packInstaller: svc),
      ),
    );

void main() {
  testWidgets('a candidate language with no pack offers Download with size',
      (t) async {
    final fake = _FakeInstaller(installed: false);
    await t.pumpWidget(_wrap(fake));
    await t.pumpAndSettle();

    expect(find.text('日本語'), findsOneWidget);
    expect(find.textContaining('Download'), findsOneWidget);
    expect(find.textContaining('MB'), findsOneWidget); // ≈18 MB
  });

  testWidgets('tapping Download installs the pack, then shows Remove',
      (t) async {
    final fake = _FakeInstaller(installed: false);
    await t.pumpWidget(_wrap(fake));
    await t.pumpAndSettle();

    await t.tap(find.textContaining('Download'));
    await t.pumpAndSettle();

    expect(fake.installCalledFor, 'ja');
    expect(find.textContaining('Remove'), findsOneWidget);
  });

  testWidgets('an already-installed pack shows Remove and can be removed',
      (t) async {
    final fake = _FakeInstaller(installed: true);
    await t.pumpWidget(_wrap(fake));
    await t.pumpAndSettle();

    expect(find.textContaining('Remove'), findsOneWidget);
    await t.tap(find.textContaining('Remove'));
    await t.pumpAndSettle();

    expect(fake.removeCalledFor, 'ja');
    expect(find.textContaining('Download'), findsOneWidget);
  });
}

class _FakeInstaller implements PackInstaller {
  bool installed;
  String? installCalledFor;
  String? removeCalledFor;
  _FakeInstaller({this.installed = false});

  @override
  Future<bool> isInstalled(String packId) async => installed;

  @override
  Future<String> install({
    required String packId,
    required String url,
    required String sha256,
    int? sizeBytes,
    void Function(double progress)? onProgress,
  }) async {
    installCalledFor = packId;
    onProgress?.call(1.0);
    installed = true;
    return '/tmp/$packId.pack';
  }

  @override
  Future<void> remove(String packId) async {
    removeCalledFor = packId;
    installed = false;
  }
}
