import Foundation
import AppKit

/// Orchestrates upgrade-to-Pro + subscription-management on macOS.
///
/// Owns the {kind → handler} map (strategy pattern mirroring the
/// Flutter client and backend), exposes high-level operations the
/// UI calls, and produces status update streams the QR / bank
/// sheets subscribe to.
@MainActor
final class PaymentService {
    let client: PaymentClient
    private var handlers: [PaymentMethodKind: PaymentHandler] = [:]

    init(client: PaymentClient) {
        self.client = client
        registerDefaultHandlers()
    }

    private func registerDefaultHandlers() {
        let watch: (String) -> AsyncStream<PaymentStatusUpdate> = { [weak self] ref in
            guard let self else {
                return AsyncStream { $0.finish() }
            }
            return self.watchPayment(referenceCode: ref)
        }
        let entries: [PaymentHandler] = [
            RedirectPaymentHandler(kind: .lemonsqueezy),
            RedirectPaymentHandler(kind: .stripe),
            RedirectPaymentHandler(kind: .paypal),
            QrPaymentHandler(statusWatcher: watch),
            BankTransferPaymentHandler(statusWatcher: watch),
        ]
        for h in entries { handlers[h.kind] = h }
    }

    /// Override the handler for one kind (used in tests + future
    /// platform-specific overrides).
    func register(_ handler: PaymentHandler) {
        handlers[handler.kind] = handler
    }

    // MARK: - Public API

    /// Methods the user can pick from.  No Apple-store policy gate
    /// on macOS — show everything the backend enables.
    func listAvailableMethods() async throws -> [PaymentMethodKind] {
        return try await client.listPaymentMethods()
    }

    /// Resolve the Pro-tier plan id from `/plans` for the requested
    /// [method] + [billingPeriod].
    ///
    ///   - Currency-aware so VietQR doesn't pick a USD plan (which
    ///     would bake "$4.99 đồng" into the QR — useless).
    ///   - Cadence-aware so the yearly toggle on the subscription
    ///     view actually charges the yearly variant.
    ///
    /// Single source of truth so the UI doesn't carry plan-picking
    /// logic.  Mirrors `PaymentService.resolveProPlanId` on the
    /// Flutter client.
    func resolveProPlanId(
        method: PaymentMethodKind? = nil,
        billingPeriod: BillingPeriod? = nil
    ) async throws -> String {
        let plans = try await client.listPlans()
        let currency = method.flatMap { Self.currency(for: $0) }
        let paid = plans.filter { p in
            let bp = (p["billing_period"] as? String)?.lowercased() ?? ""
            let active = (p["is_active"] as? Bool) ?? true
            if bp.isEmpty || bp == "none" || !active { return false }
            if let want = currency {
                let c = (p["currency"] as? String)?.uppercased() ?? ""
                if c != want.uppercased() { return false }
            }
            return true
        }
        guard !paid.isEmpty else { throw PaymentError.noProPlan }
        if let want = billingPeriod,
           let exact = paid.first(where: {
               BillingPeriod.fromWire($0["billing_period"] as? String) == want
           }),
           let id = exact["id"] as? String, !id.isEmpty {
            return id
        }
        // No exact cadence match (or none requested) — fall back to
        // monthly, then the first paid plan.
        let monthly = paid.first(where: {
            BillingPeriod.fromWire($0["billing_period"] as? String) == .monthly
        }) ?? paid[0]
        guard let id = monthly["id"] as? String, !id.isEmpty else {
            throw PaymentError.noProPlan
        }
        return id
    }

    /// Currency the strategy expects to charge the plan in.  VietQR +
    /// bank-transfer can only settle in VND because the QR code is a
    /// Vietnamese-bank-only spec; everything else defaults to USD.
    /// Mirrors `_currencyFor` on the Flutter client.
    static func currency(for method: PaymentMethodKind) -> String {
        switch method {
        case .vietqr, .bankTransfer: return "VND"
        case .lemonsqueezy, .stripe, .paypal: return "USD"
        }
    }

    /// Run the full upgrade flow for [method]: create checkout
    /// server-side, then dispatch to the registered handler.
    func upgrade(method: PaymentMethodKind, planId: String, presenter: PaymentSheetPresenter) async throws {
        guard let handler = handlers[method] else {
            throw PaymentError.notConfigured
        }
        let result = try await client.createCheckout(planId: planId, method: method)
        DRLogger.log("checkout created: method=\(method.wireName) ref=\(result.referenceCode)", category: .api)
        try await handler.handle(result, presenter: presenter)
    }

    /// Open the Lemon Squeezy Customer Portal in the user's browser.
    func openCustomerPortal() async throws {
        let url = try await client.getCustomerPortalURL()
        if !NSWorkspace.shared.open(url) {
            throw PaymentError.http(0, "Could not open the browser")
        }
    }

    // MARK: - Foreground status poller

    /// Poll `/payment/status/:ref` until terminal, [timeout] elapses,
    /// or the consumer drops the stream.  Yields one update per
    /// poll so the banner can render live state.
    func watchPayment(
        referenceCode: String,
        interval: TimeInterval = 3,
        timeout: TimeInterval = 60 * 15
    ) -> AsyncStream<PaymentStatusUpdate> {
        AsyncStream { continuation in
            let task = Task { [client] in
                let deadline = Date().addingTimeInterval(timeout)
                while !Task.isCancelled && Date() < deadline {
                    do {
                        let update = try await client.getPaymentStatus(referenceCode: referenceCode)
                        continuation.yield(update)
                        if update.status.isTerminal {
                            continuation.finish()
                            return
                        }
                    } catch {
                        DRLogger.warn("status poll failed: \(error)", category: .api)
                    }
                    try? await Task.sleep(nanoseconds: UInt64(interval * 1_000_000_000))
                }
                // Deadline exceeded — emit a synthetic expired update.
                continuation.yield(PaymentStatusUpdate(json: [
                    "reference_code": referenceCode,
                    "status": "expired",
                ]))
                continuation.finish()
            }
            continuation.onTermination = { _ in task.cancel() }
        }
    }
}
