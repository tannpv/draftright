import 'package:flutter/material.dart';
import 'package:draftright_mobile/services/payment/billing_period.dart';

/// Segmented Monthly / Yearly toggle shown above the payment-method
/// list on the Subscription screen.  Pure presentation — the parent
/// owns the current value and threads it into
/// `PaymentService.resolveProPlanId(billingPeriod: …)`.
///
/// Lives in `widgets/` rather than inline in `subscription_screen.dart`
/// so the desktop wrappers (when ported) and any future "upgrade"
/// surface can reuse the exact same affordance.
class BillingPeriodSelector extends StatelessWidget {
  final BillingPeriod value;
  final ValueChanged<BillingPeriod> onChanged;

  /// When true, render as a centered dense pill row suitable for
  /// dialogs.  Default: stretched segmented control suitable for a
  /// settings page card.
  final bool compact;

  const BillingPeriodSelector({
    super.key,
    required this.value,
    required this.onChanged,
    this.compact = false,
  });

  @override
  Widget build(BuildContext context) {
    final segments = BillingPeriod.values
        .map((p) => ButtonSegment<BillingPeriod>(
              value: p,
              label: Text(p.displayName),
            ))
        .toList();
    final picker = SegmentedButton<BillingPeriod>(
      segments: segments,
      selected: {value},
      showSelectedIcon: false,
      onSelectionChanged: (s) {
        if (s.isNotEmpty) onChanged(s.first);
      },
    );
    if (compact) return Center(child: picker);
    return SizedBox(width: double.infinity, child: picker);
  }
}
