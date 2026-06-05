import SwiftUI

/// Segmented Monthly / Yearly picker shown above the payment-method
/// list on the Subscription view.  Pure presentation — the parent
/// owns the current value and threads it into
/// `PaymentService.resolveProPlanId(billingPeriod:)`.
///
/// Lives next to `SubscriptionView` so any future upgrade surface
/// (the floating panel's upsell, a deep-link landing page, …) can
/// reuse the same affordance.
struct BillingPeriodPicker: View {
    @Binding var selection: BillingPeriod

    var body: some View {
        Picker("", selection: $selection) {
            ForEach(BillingPeriod.allCases, id: \.self) { period in
                Text(period.displayName).tag(period)
            }
        }
        .pickerStyle(.segmented)
        .labelsHidden()
    }
}
