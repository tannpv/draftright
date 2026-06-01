import 'package:flutter_test/flutter_test.dart';
import 'package:draftright_mobile/services/payment/vn_bank_apps.dart';

/// Fake launcher used to exercise the registry without touching
/// url_launcher.
class _FakeLauncher implements BankAppLauncher {
  @override final String code;
  @override final String displayName;
  final BankAppLaunchOutcome outcome;
  int callCount = 0;

  _FakeLauncher({
    required this.code,
    required this.displayName,
    this.outcome = BankAppLaunchOutcome.appOpened,
  });

  @override
  Future<BankAppLaunchOutcome> launch({BankAppLaunchContext? context}) async {
    callCount++;
    return outcome;
  }
}

void main() {
  group('BankAppRegistry.forVietnam', () {
    final reg = BankAppRegistry.forVietnam();

    test('returns at least the 7 canonical VN banks', () {
      final codes = reg.all().map((l) => l.code).toSet();
      expect(codes.containsAll({'MB', 'ACB', 'VCB', 'AB', 'TPB', 'TCB', 'VTB'}), isTrue,
          reason: 'missing one of the canonical VN bank codes: $codes');
    });

    test('every entry has a non-empty displayName', () {
      for (final l in reg.all()) {
        expect(l.displayName.isNotEmpty, isTrue, reason: '${l.code} has empty displayName');
      }
    });

    test('findByCode returns the matching launcher', () {
      expect(reg.findByCode('MB')?.displayName, contains('MB'));
      expect(reg.findByCode('VCB')?.displayName, contains('Vietcombank'));
    });

    test('findByCode returns null for unknown code', () {
      expect(reg.findByCode('XYZ'), isNull);
      expect(reg.findByCode(''), isNull);
    });

    test('all() is immutable (defensive copy)', () {
      final list = reg.all();
      expect(() => list.add(_FakeLauncher(code: 'X', displayName: 'X')),
          throwsA(isA<UnsupportedError>()));
    });
  });

  group('Custom registry with fake launchers', () {
    test('iterates injected launchers in order', () {
      final a = _FakeLauncher(code: 'A', displayName: 'Alpha');
      final b = _FakeLauncher(code: 'B', displayName: 'Beta');
      final reg = BankAppRegistry([a, b]);
      expect(reg.all().map((l) => l.code).toList(), ['A', 'B']);
    });

    test('launches via the strategy each registered launcher chose', () async {
      final a = _FakeLauncher(code: 'A', displayName: 'Alpha');
      final reg = BankAppRegistry([a]);
      final l = reg.findByCode('A')!;
      final outcome = await l.launch();
      expect(outcome, BankAppLaunchOutcome.appOpened);
      expect(a.callCount, 1);
    });
  });

  group('UrlSchemeBankAppLauncher properties', () {
    test('exposes code, displayName, scheme, package', () {
      const l = UrlSchemeBankAppLauncher(
        code: 'TEST',
        displayName: 'Test Bank',
        urlScheme: 'test://',
        androidPackage: 'com.test.bank',
      );
      expect(l.code, 'TEST');
      expect(l.displayName, 'Test Bank');
      expect(l.urlScheme, 'test://');
      expect(l.androidPackage, 'com.test.bank');
    });
  });
}
