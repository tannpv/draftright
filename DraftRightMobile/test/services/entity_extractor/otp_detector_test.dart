import 'package:flutter_test/flutter_test.dart';
import 'package:draftright_mobile/services/entity_extractor/otp_detector.dart';
import 'package:draftright_mobile/models/entity.dart';

void main() {
  final det = OtpDetector();
  test('detects when trigger word "OTP" present', () {
    final r = det.detect('Your OTP is 482917 for login');
    expect(r.single.kind, EntityKind.otp);
    expect(r.single.value, '482917');
  });

  test('detects with Vietnamese trigger "mã"', () {
    final r = det.detect('Mã xác minh: 4829');
    expect(r.single.value, '4829');
  });

  test('detects with "mật khẩu" (password)', () {
    final r = det.detect('Mật khẩu wifi 88889999');
    expect(r.single.value, '88889999');
  });

  test('rejects bare 4-digit year (no trigger)', () {
    expect(det.detect('Năm 2024 là năm tốt'), isEmpty);
  });

  test('rejects phone-shaped numbers (10 digits)', () {
    expect(det.detect('Call 0987654321'), isEmpty);
  });
}
