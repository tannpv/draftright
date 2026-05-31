import 'package:flutter_test/flutter_test.dart';
import 'package:draftright_mobile/services/payment/payment_status.dart';

void main() {
  group('PaymentStatus.fromWire', () {
    test('maps every backend enum value', () {
      expect(PaymentStatus.fromWire('pending'),   PaymentStatus.pending);
      expect(PaymentStatus.fromWire('completed'), PaymentStatus.completed);
      expect(PaymentStatus.fromWire('failed'),    PaymentStatus.failed);
      expect(PaymentStatus.fromWire('expired'),   PaymentStatus.expired);
      expect(PaymentStatus.fromWire('refunded'),  PaymentStatus.refunded);
    });

    test('synthetic not_found from /payment/status/:ref maps cleanly', () {
      expect(PaymentStatus.fromWire('not_found'), PaymentStatus.notFound);
    });

    test('unknown strings → PaymentStatus.unknown (forward-compat)', () {
      expect(PaymentStatus.fromWire('disputed'), PaymentStatus.unknown);
      expect(PaymentStatus.fromWire(''),         PaymentStatus.unknown);
    });
  });

  group('PaymentStatus.isTerminal', () {
    test('non-terminal: pending, notFound, unknown', () {
      expect(PaymentStatus.pending.isTerminal,  false);
      expect(PaymentStatus.notFound.isTerminal, false);
      expect(PaymentStatus.unknown.isTerminal,  false);
    });

    test('terminal: completed, failed, expired, refunded', () {
      expect(PaymentStatus.completed.isTerminal, true);
      expect(PaymentStatus.failed.isTerminal,    true);
      expect(PaymentStatus.expired.isTerminal,   true);
      expect(PaymentStatus.refunded.isTerminal,  true);
    });

    test('isSuccess only true for completed', () {
      expect(PaymentStatus.completed.isSuccess, true);
      expect(PaymentStatus.pending.isSuccess,   false);
      expect(PaymentStatus.failed.isSuccess,    false);
      expect(PaymentStatus.expired.isSuccess,   false);
    });
  });

  group('PaymentStatusUpdate.fromJson', () {
    test('parses controller envelope from /payment/status/:ref', () {
      final u = PaymentStatusUpdate.fromJson({
        'status': 'completed',
        'method': 'vietqr',
        'amount': 99000,
        'currency': 'VND',
        'reference_code': 'PAY-ABC',
        'plan_name': 'Pro',
        'completed_at': '2026-05-31T13:24:00.000Z',
        'expires_at':   '2026-05-31T14:00:00.000Z',
      });
      expect(u.status, PaymentStatus.completed);
      expect(u.referenceCode, 'PAY-ABC');
      expect(u.amount, 99000);
      expect(u.currency, 'VND');
      expect(u.planName, 'Pro');
      expect(u.completedAt, isNotNull);
      expect(u.expiresAt, isNotNull);
    });

    test('synthetic not_found envelope (status only) parses cleanly', () {
      // PaymentController returns just `{status: 'not_found'}` when
      // the reference doesn't match any row.
      final u = PaymentStatusUpdate.fromJson({'status': 'not_found'});
      expect(u.status, PaymentStatus.notFound);
      expect(u.referenceCode, '');
      expect(u.amount, isNull);
    });

    test('non-string date fields → null instead of throwing', () {
      final u = PaymentStatusUpdate.fromJson({
        'status': 'pending',
        'completed_at': null,
      });
      expect(u.completedAt, isNull);
    });
  });
}
