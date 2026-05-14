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
}
