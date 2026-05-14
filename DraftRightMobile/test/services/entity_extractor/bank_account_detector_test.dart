import 'package:flutter_test/flutter_test.dart';
import 'package:draftright_mobile/services/entity_extractor/bank_account_detector.dart';
import 'package:draftright_mobile/models/entity.dart';

void main() {
  final det = BankAccountDetector();

  test('detects Vietcombank account', () {
    final r = det.detect('Chuyển khoản Vietcombank 0123456789 giúp anh');
    expect(r.single.kind, EntityKind.bankAccount);
    expect(r.single.value, '0123456789');
    expect(r.single.meta['bank'], 'Vietcombank');
    expect(r.single.display, 'Vietcombank · 0123456789');
  });

  test('detects MB Bank lowercase', () {
    final r = det.detect('mb 9876543210');
    expect(r.single.meta['bank'], 'MB');
  });

  test('rejects standalone numbers without bank context', () {
    expect(det.detect('Số là 0123456789'), isEmpty);
  });

  test('rejects bank name without nearby account', () {
    expect(
      det.detect('Tôi xài Vietcombank rất nhiều, nhưng không nhớ số tài khoản'),
      isEmpty,
    );
  });
}
