// Unit tests for PaymentService.resolveProPlanId — the plan lookup that runs
// when a user taps a payment method on the Subscription screen.
//
// Regression guard for BUG-42: when the backend's plan catalog had no
// matching Pro plan, the resolver threw an Exception whose text leaked the
// raw Dart enum ("Could not find a Pro plan in USD (monthly) for
// PaymentMethodKind.lemonsqueezy") straight into a user-facing snackbar. The
// fix logs the technical detail and throws a friendly, enum-free message.

import 'package:flutter_test/flutter_test.dart';

import 'package:draftright_mobile/services/auth_service.dart';
import 'package:draftright_mobile/services/backend_client.dart';
import 'package:draftright_mobile/services/payment/billing_period.dart';
import 'package:draftright_mobile/services/payment/payment_method.dart';
import 'package:draftright_mobile/services/payment_service.dart';

/// BackendClient whose plan catalog is fixed by the test. Only listPlans() is
/// exercised by resolveProPlanId, so the rest of the client is never touched.
class _FakeBackend extends BackendClient {
  final List<Map<String, dynamic>> _plans;
  _FakeBackend(this._plans)
      : super(auth: AuthService(), getBaseUrl: () => 'http://localhost');
  @override
  Future<List<Map<String, dynamic>>> listPlans() async => _plans;
}

void main() {
  test('returns the matching plan id when a Pro/USD/monthly plan exists',
      () async {
    final svc = PaymentService(_FakeBackend([
      {'name': 'Free', 'billing_period': 'none', 'is_active': true},
      {
        'id': 'pro-usd-monthly',
        'name': 'Pro',
        'currency': 'USD',
        'billing_period': 'monthly',
        'is_active': true,
      },
    ]));

    final id = await svc.resolveProPlanId(
      method: PaymentMethodKind.lemonsqueezy,
      billingPeriod: BillingPeriod.monthly,
    );

    expect(id, 'pro-usd-monthly');
  });

  test('throws a friendly, enum-free message when no plan matches (BUG-42)',
      () async {
    // Only Free — no paid Pro plan, mirroring the misconfigured dev catalog
    // that produced BUG-42.
    final svc = PaymentService(_FakeBackend([
      {'name': 'Free', 'billing_period': 'none', 'is_active': true},
    ]));

    await expectLater(
      svc.resolveProPlanId(
        method: PaymentMethodKind.lemonsqueezy,
        billingPeriod: BillingPeriod.monthly,
      ),
      throwsA(predicate((e) {
        final msg = e.toString();
        // Must NOT leak the raw enum...
        final noEnum = !msg.contains('PaymentMethodKind');
        // ...and SHOULD name the method in human terms.
        final friendly = msg.contains('Credit / Debit Card');
        return e is Exception && noEnum && friendly;
      })),
    );
  });
}
