import SwiftUI

/// Main Subscription screen, hosted inside a Settings tab.  Mirrors
/// the Flutter SubscriptionScreen: shows plan + usage, lists payment
/// methods for free users, shows the Manage button for paid users.
///
/// The actor model on macOS doesn't have an `AppLifecycleState`-style
/// resume hook, but Settings windows are re-focused often enough
/// that an explicit Refresh button + auto-refresh on `.onAppear`
/// covers the post-checkout return-to-app case.
struct SubscriptionView: View {
    @EnvironmentObject var appModel: AppModel
    @StateObject private var vm = SubscriptionViewModel()

    // Active sheet (QR or bank-transfer) — driven by the
    // PaymentSheetPresenter callbacks registered on the view model.
    @State private var activeSheet: ActiveSheet?

    var body: some View {
        ScrollView {
            VStack(alignment: .leading, spacing: 16) {
                Header(isLoggedIn: appModel.isLoggedIn)
                if !appModel.isLoggedIn {
                    Text("Sign in (in the Account tab) to view your subscription.")
                        .foregroundColor(.secondary)
                } else {
                    body(forLoggedIn: vm)
                }
            }
            .padding()
        }
        .onAppear {
            if appModel.isLoggedIn {
                vm.bind(appModel: appModel)
                Task { await vm.refresh() }
            }
        }
        // 1-arg `.onChange` signature kept for macOS 13 compat
        // (Package.swift target: `.macOS(.v13)`).  The 2-arg
        // `(oldValue, newValue)` form is macOS 14+.
        .onChange(of: appModel.isLoggedIn) { loggedIn in
            if loggedIn {
                vm.bind(appModel: appModel)
                Task { await vm.refresh() }
            } else {
                vm.reset()
            }
        }
        .onReceive(vm.$pendingSheet.compactMap { $0 }) { sheet in
            activeSheet = sheet
        }
        .sheet(item: $activeSheet) { sheet in
            sheetView(for: sheet)
        }
    }

    @ViewBuilder
    private func body(forLoggedIn vm: SubscriptionViewModel) -> some View {
        switch vm.state {
        case .loading:
            ProgressView().padding()
        case .error(let message):
            VStack(alignment: .leading, spacing: 8) {
                Label(message, systemImage: "exclamationmark.triangle.fill")
                    .foregroundColor(.red)
                Button("Retry") { Task { await vm.refresh() } }
            }
        case .loaded(let info):
            InfoCards(info: info)
            Divider()
            if info.isFree {
                upgradeBlock
            } else {
                manageBlock
            }
        }
    }

    @ViewBuilder
    private var upgradeBlock: some View {
        VStack(alignment: .leading, spacing: 6) {
            Text("Upgrade to Pro").font(.title3.weight(.semibold))
            Text("Pick a billing cadence, then a payment method.  Your plan activates automatically once payment completes.")
                .font(.callout)
                .foregroundColor(.secondary)
        }
        BillingPeriodPicker(selection: $vm.billingPeriod)
        Group {
            if vm.methodsLoaded {
                if vm.availableMethods.isEmpty {
                    Text("No payment methods are enabled yet.  Please check back later.")
                        .foregroundColor(.secondary)
                } else {
                    MethodPickerView(
                        methods: vm.availableMethods,
                        isStarting: vm.isStarting,
                        startingKind: vm.startingKind,
                        onSelect: { kind in Task { await vm.upgrade(kind) } }
                    )
                }
            } else {
                HStack {
                    ProgressView().controlSize(.small)
                    Text("Loading payment methods…")
                        .foregroundColor(.secondary)
                }
            }
        }
    }

    @ViewBuilder
    private var manageBlock: some View {
        VStack(alignment: .leading, spacing: 8) {
            Button {
                Task { await vm.openCustomerPortal() }
            } label: {
                if vm.isOpeningPortal {
                    HStack(spacing: 8) {
                        ProgressView().controlSize(.small)
                        Text("Opening…")
                    }
                } else {
                    Label("Manage subscription", systemImage: "gearshape")
                }
            }
            .controlSize(.large)
            .disabled(vm.isOpeningPortal)

            Text("Cancel, change plan, or update your payment method.")
                .font(.caption)
                .foregroundColor(.secondary)
        }
    }

    @ViewBuilder
    private func sheetView(for sheet: ActiveSheet) -> some View {
        switch sheet {
        case .qr(let url, let bank, let stream):
            QrCheckoutSheet(
                imageURL: url,
                bankInfo: bank,
                statusStream: stream,
                onClose: {
                    activeSheet = nil
                    Task { await vm.refresh() }
                }
            )
        case .bank(let info, let stream):
            BankTransferSheet(
                info: info,
                statusStream: stream,
                onClose: {
                    activeSheet = nil
                    Task { await vm.refresh() }
                }
            )
        }
    }
}

// MARK: - Active-sheet identity

enum ActiveSheet: Identifiable {
    case qr(URL, BankInfo?, AsyncStream<PaymentStatusUpdate>?)
    case bank(BankInfo, AsyncStream<PaymentStatusUpdate>?)

    var id: String {
        switch self {
        case .qr(let url, _, _):  return "qr-\(url.absoluteString)"
        case .bank(let info, _):  return "bank-\(info.reference)"
        }
    }
}

// MARK: - Sub-views

private struct Header: View {
    let isLoggedIn: Bool
    var body: some View {
        HStack {
            Image(systemName: "creditcard.and.123")
            Text("Subscription").font(.title2.weight(.semibold))
            Spacer()
        }
    }
}

private struct InfoCards: View {
    let info: SubscriptionViewModel.Info

    var body: some View {
        VStack(alignment: .leading, spacing: 8) {
            row("Plan",    info.planName)
            row("Billing", info.billingLabel)
            row("Status",  info.statusLabel)
            if let expires = info.expiresAt {
                row("Expires", expires)
            }
            row("Usage today", "\(info.usageToday) / \(info.dailyLimit)")
        }
    }

    @ViewBuilder
    private func row(_ label: String, _ value: String) -> some View {
        HStack {
            Text(label).foregroundColor(.secondary).frame(width: 110, alignment: .leading)
            Text(value).fontWeight(.medium)
            Spacer()
        }
    }
}
