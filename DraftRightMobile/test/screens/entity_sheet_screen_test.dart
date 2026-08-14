import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:draftright_mobile/models/entity.dart';
import 'package:draftright_mobile/screens/entity_sheet_screen.dart';

void main() {
  testWidgets('renders one group per kind, copy button works', (tester) async {
    final initial = [
      Entity(
        kind: EntityKind.phone,
        value: '+84912345678',
        display: '0912 345 678',
        start: 0,
        end: 11,
        source: 'regex',
        confidence: 1.0,
      ),
      Entity(
        kind: EntityKind.email,
        value: 'tan@x.com',
        display: 'tan@x.com',
        start: 15,
        end: 24,
        source: 'regex',
        confidence: 1.0,
      ),
    ];

    String? copied;
    TestDefaultBinaryMessengerBinding.instance.defaultBinaryMessenger
        .setMockMethodCallHandler(SystemChannels.platform, (call) async {
      if (call.method == 'Clipboard.setData') {
        copied = (call.arguments as Map)['text'] as String;
      }
      return null;
    });

    await tester.pumpWidget(MaterialApp(
      home: EntitySheetScreen(
        text: 'hello',
        initial: initial,
        smartScan: null,
      ),
    ));

    expect(find.text('0912 345 678'), findsOneWidget);
    expect(find.text('tan@x.com'), findsOneWidget);
    expect(find.text('Phone'), findsOneWidget);
    expect(find.text('Email'), findsOneWidget);

    await tester.tap(find.byKey(const ValueKey('copy-phone-+84912345678')));
    await tester.pump();
    expect(copied, '+84912345678');
  });

  testWidgets('credit card row shows masked display by default', (tester) async {
    await tester.pumpWidget(MaterialApp(
      home: EntitySheetScreen(
        text: '4242 4242 4242 4242',
        initial: [
          Entity(
            kind: EntityKind.creditCard,
            value: '4242424242424242',
            display: '**** **** **** 4242',
            start: 0,
            end: 19,
            source: 'regex',
            confidence: 1.0,
            meta: const {'masked': 'true'},
          ),
        ],
        smartScan: null,
      ),
    ));
    expect(find.text('**** **** **** 4242'), findsOneWidget);
  });

  testWidgets('smart scan dedupes LLM entities matching regex by (kind, value)', (tester) async {
    final regex = [
      Entity(
        kind: EntityKind.phone,
        value: '+84912345678',
        display: '0912 345 678',
        start: 0,
        end: 11,
        source: 'regex',
        confidence: 0.95,
      ),
    ];
    // LLM returns the same phone (already detected by regex) PLUS an address.
    Future<List<Entity>> fakeSmartScan(String _) async => [
          Entity(
            kind: EntityKind.phone,
            value: '+84912345678',
            display: '+84912345678',
            start: 0,
            end: 12,
            source: 'llm',
            confidence: 0.8,
          ),
          Entity(
            kind: EntityKind.address,
            value: '123 Lê Lợi',
            display: '123 Lê Lợi',
            start: 20,
            end: 30,
            source: 'llm',
            confidence: 0.85,
          ),
        ];

    await tester.pumpWidget(MaterialApp(
      home: EntitySheetScreen(
        text: '0912 345 678 ... 123 Lê Lợi',
        initial: regex,
        smartScan: fakeSmartScan,
      ),
    ));
    expect(find.text('Phone'), findsOneWidget);
    expect(find.text('Address'), findsNothing);  // not yet — smart scan not triggered

    // Tap Smart scan
    await tester.tap(find.text('Smart scan for addresses, names…'));
    await tester.pumpAndSettle();

    // Phone group should still have exactly 1 row (no duplicate)
    expect(find.text('0912 345 678'), findsOneWidget);
    // Address row now appears
    expect(find.text('123 Lê Lợi'), findsOneWidget);
  });

  group('entityActionFor', () {
    Entity e(EntityKind k, String v) => Entity(
        kind: k, value: v, display: v, start: 0, end: v.length, source: 'regex', confidence: 1.0);

    test('phone → tel: URI', () {
      final a = entityActionFor(e(EntityKind.phone, '+84912345678'));
      expect(a?.uri, Uri(scheme: 'tel', path: '+84912345678'));
    });
    test('email → mailto: URI', () {
      final a = entityActionFor(e(EntityKind.email, 'tan@x.com'));
      expect(a?.uri.scheme, 'mailto');
      expect(a?.uri.path, 'tan@x.com');
    });
    test('url without scheme gets https://', () {
      final a = entityActionFor(e(EntityKind.url, 'draftright.info'));
      expect(a?.uri.toString(), 'https://draftright.info');
    });
    test('url with scheme is unchanged', () {
      final a = entityActionFor(e(EntityKind.url, 'http://x.com/y'));
      expect(a?.uri.toString(), 'http://x.com/y');
    });
    test('address → google maps search with encoded query', () {
      final a = entityActionFor(e(EntityKind.address, '123 Lê Lợi'));
      expect(a?.uri.host, 'www.google.com');
      expect(a?.uri.toString(), contains(Uri.encodeComponent('123 Lê Lợi')));
    });
    test('copy-only kinds have no action', () {
      expect(entityActionFor(e(EntityKind.otp, '123456')), isNull);
      expect(entityActionFor(e(EntityKind.bankAccount, '0123')), isNull);
      expect(entityActionFor(e(EntityKind.creditCard, '4111')), isNull);
      expect(entityActionFor(e(EntityKind.personName, 'Tan')), isNull);
    });
  });
}
