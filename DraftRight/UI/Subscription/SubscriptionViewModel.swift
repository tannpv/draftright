import Foundation
import SwiftUI

/// State machine + glue between [PaymentService] and the SwiftUI
/// [SubscriptionView].  Keeps view logic small (data + actions) and
/// makes the service easy to fake in previews.
@MainActor
final class SubscriptionViewModel: ObservableObject, PaymentSheetPresenter {
    enum State {
        case loading
        case loaded(Info)
        case error(String)
    }

    struct Info {
        let planName: String
        let billingLabel: String
        let statusLabel: String
        let expiresAt: String?
        let usageToday: Int
        let dailyLimit: Int
        let isFree: Bool
    }

    @Published var state: State = .loading
    @Published var availableMethods: [PaymentMethodKind] = []
    @Published var methodsLoaded: Bool = false
    @Published var isStarting: Bool = false
    @Published var startingKind: PaymentMethodKind?
    @Published var isOpeningPortal: Bool = false
    @Published var pendingSheet: ActiveSheet?

    /// User-selected billing cadence for the upgrade button.  Defaults
    /// to monthly (lower friction, lower commitment).  Threaded into
    /// `PaymentService.resolveProPlanId` so the backend creates a
    /// checkout for the matching plan id.
    @Published var billingPeriod: BillingPeriod = .monthly

    private var service: PaymentService?

    func bind(appModel: AppModel) {
        // Construct the service if not bound yet — re-binding on
        // logout/login is idempotent because PaymentService is cheap.
        let client = PaymentClient(appModel: appModel)
        self.service = PaymentService(client: client)
    }

    func reset() {
        state = .loading
        availableMethods = []
        methodsLoaded = false
        pendingSheet = nil
    }

    // MARK: - Refresh

    func refresh() async {
        guard let service else { return }
        state = .loading
        do {
            async let subRaw = service.client.getSubscription()
            async let methods = service.listAvailableMethods()
            let (json, kinds) = try await (subRaw, methods)
            state = .loaded(Self.decodeInfo(json: json))
            availableMethods = kinds
            methodsLoaded = true
        } catch {
            state = .error(error.localizedDescription)
            methodsLoaded = true
        }
    }

    // MARK: - Upgrade

    func upgrade(_ kind: PaymentMethodKind) async {
        guard let service, !isStarting else { return }
        isStarting = true
        startingKind = kind
        defer { isStarting = false; startingKind = nil }
        do {
            // Pass method + cadence so the resolver picks a
            // currency-compatible plan (VND for VietQR/bank, USD
            // otherwise) at the cadence the user toggled.  See the
            // LS yearly-fix tripod in [[project_cc_payment_lemonsqueezy]].
            let planId = try await service.resolveProPlanId(
                method: kind,
                billingPeriod: billingPeriod
            )
            try await service.upgrade(method: kind, planId: planId, presenter: self)
        } catch {
            state = .error(error.localizedDescription)
        }
    }

    // MARK: - Customer portal

    func openCustomerPortal() async {
        guard let service, !isOpeningPortal else { return }
        isOpeningPortal = true
        defer { isOpeningPortal = false }
        do {
            try await service.openCustomerPortal()
        } catch {
            state = .error(error.localizedDescription)
        }
    }

    // MARK: - PaymentSheetPresenter

    func presentQrSheet(_ result: CheckoutResult, statusStream: AsyncStream<PaymentStatusUpdate>?) {
        guard case .qr(_, let url, let bank) = result else { return }
        pendingSheet = .qr(url, bank, statusStream)
    }

    func presentBankTransferSheet(_ result: CheckoutResult, statusStream: AsyncStream<PaymentStatusUpdate>?) {
        guard case .bankTransfer(_, let info) = result else { return }
        pendingSheet = .bank(info, statusStream)
    }

    // MARK: - JSON decoding

    static func decodeInfo(json: [String: Any]) -> Info {
        var planName = "Free"
        var billingPeriod = "none"
        var dailyLimit = 10
        if let plan = json["plan"] as? [String: Any] {
            planName = (plan["name"] as? String) ?? "Free"
            billingPeriod = (plan["billing_period"] as? String) ?? "none"
            dailyLimit = (plan["daily_limit"] as? Int) ?? 10
        } else if let s = json["plan"] as? String {
            planName = s
        }

        let billingLabel: String = {
            switch billingPeriod {
            case "none":    return "Free"
            case "monthly": return "Monthly"
            case "yearly":  return "Yearly"
            default:        return billingPeriod
            }
        }()

        let status = (json["status"] as? String) ?? "active"
        let statusLabel: String = {
            switch status {
            case "active":    return "Active"
            case "expired":   return "Expired"
            case "cancelled": return "Cancelled"
            default:          return status
            }
        }()

        return Info(
            planName: planName,
            billingLabel: billingLabel,
            statusLabel: statusLabel,
            expiresAt: json["expires_at"] as? String,
            usageToday: (json["usage_today"] as? Int) ?? 0,
            dailyLimit: dailyLimit,
            isFree: billingPeriod == "none"
        )
    }
}
