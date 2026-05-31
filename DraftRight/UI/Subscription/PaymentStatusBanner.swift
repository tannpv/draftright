import SwiftUI

/// Compact live-status banner shown inside QR / bank-transfer sheets.
///
/// Subscribes to an [AsyncStream] of [PaymentStatusUpdate] (the
/// `PaymentService.watchPayment` poller) and renders one of three
/// visual states:
///
///   - **pending** — spinner + "Waiting for payment…"
///   - **success** — green check + "Payment confirmed!", then auto-
///     dismisses the enclosing sheet via the [onConfirmed] callback
///     after [autoDismissDelay].
///   - **failure** — red icon + reason string; stays visible until
///     the user closes the sheet.
///
/// A nil [stream] makes the banner a no-op so the sheets render
/// the same way in tests / when the watcher is unavailable.
struct PaymentStatusBanner: View {
    let stream: AsyncStream<PaymentStatusUpdate>?
    var autoDismissDelay: TimeInterval = 2.0
    var onConfirmed: () -> Void = {}

    @State private var latest: PaymentStatusUpdate?

    var body: some View {
        Group {
            if stream != nil {
                bannerView
            } else {
                EmptyView()
            }
        }
        .task(id: ObjectIdentifier(stream as AnyObject)) {
            guard let stream else { return }
            for await update in stream {
                latest = update
                if update.status.isSuccess {
                    try? await Task.sleep(nanoseconds: UInt64(autoDismissDelay * 1_000_000_000))
                    onConfirmed()
                }
            }
        }
    }

    @ViewBuilder
    private var bannerView: some View {
        let (bg, fg, icon, text) = appearance(for: latest?.status ?? .pending)
        HStack(spacing: 10) {
            icon
            Text(text)
                .font(.caption.weight(.semibold))
                .foregroundColor(fg)
            Spacer()
        }
        .padding(10)
        .background(bg)
        .cornerRadius(8)
    }

    private func appearance(for status: PaymentStatus) -> (Color, Color, AnyView, String) {
        switch status {
        case .pending, .notFound, .unknown:
            return (
                Color.blue.opacity(0.10),
                .blue,
                AnyView(ProgressView().controlSize(.small).progressViewStyle(.circular)),
                "Waiting for payment…"
            )
        case .completed:
            return (
                Color.green.opacity(0.12),
                .green,
                AnyView(Image(systemName: "checkmark.circle.fill").foregroundColor(.green)),
                "Payment confirmed!"
            )
        case .failed:
            return (
                Color.red.opacity(0.12),
                .red,
                AnyView(Image(systemName: "xmark.octagon.fill").foregroundColor(.red)),
                "Payment failed. Please try again."
            )
        case .expired:
            return (
                Color.orange.opacity(0.12),
                .orange,
                AnyView(Image(systemName: "clock.badge.exclamationmark.fill").foregroundColor(.orange)),
                "Took too long to confirm. If you already paid, check Subscription in a minute."
            )
        case .refunded:
            return (
                Color.gray.opacity(0.12),
                .gray,
                AnyView(Image(systemName: "arrow.uturn.backward").foregroundColor(.gray)),
                "Refunded."
            )
        }
    }
}
