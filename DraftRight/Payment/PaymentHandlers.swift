import Foundation
import SwiftUI
import AppKit

/// Post-checkout UX for one [PaymentMethodKind].
///
/// **Why a protocol, not a switch in PaymentService:**
/// adding a method (Momo, NFC) = new struct conforming to this +
/// register in the handler map.  Subscription UI never branches on
/// method.  Mirrors Flutter's `PaymentHandler`.
@MainActor
protocol PaymentHandler {
    var kind: PaymentMethodKind { get }

    /// Drive the post-checkout flow.  Some handlers open a browser
    /// (no UI state), others present a sheet via [Presenter].
    func handle(_ result: CheckoutResult, presenter: PaymentSheetPresenter) async throws
}

/// Lightweight binding that lets handlers ask the SwiftUI host to
/// present a checkout sheet without depending on view types directly.
/// Implemented by [SubscriptionView] as a @State block.
@MainActor
protocol PaymentSheetPresenter: AnyObject {
    func presentQrSheet(_ result: CheckoutResult, statusStream: AsyncStream<PaymentStatusUpdate>?)
    func presentBankTransferSheet(_ result: CheckoutResult, statusStream: AsyncStream<PaymentStatusUpdate>?)
}

/// Opens the redirect URL in the user's default browser.  Used by
/// every URL-based provider: Lemon Squeezy, Stripe, PayPal.  On
/// macOS we can't render an in-app browser cheaply; the system
/// browser opens with the user's existing cookies, so Apple Pay /
/// saved cards from Safari still appear inside the LS checkout.
struct RedirectPaymentHandler: PaymentHandler {
    let kind: PaymentMethodKind

    func handle(_ result: CheckoutResult, presenter: PaymentSheetPresenter) async throws {
        guard case .redirect(_, let url) = result else {
            throw PaymentError.http(0, "RedirectPaymentHandler received non-redirect result")
        }
        // NSWorkspace.open returns Bool synchronously on success.
        let ok = NSWorkspace.shared.open(url)
        if !ok {
            throw PaymentError.http(0, "Could not open the browser")
        }
    }
}

/// Presents the VietQR sheet via the host's [PaymentSheetPresenter].
struct QrPaymentHandler: PaymentHandler {
    let kind: PaymentMethodKind = .vietqr
    let statusWatcher: (String) -> AsyncStream<PaymentStatusUpdate>

    func handle(_ result: CheckoutResult, presenter: PaymentSheetPresenter) async throws {
        guard case .qr(let ref, _, _) = result else {
            throw PaymentError.http(0, "QrPaymentHandler received non-qr result")
        }
        presenter.presentQrSheet(result, statusStream: statusWatcher(ref))
    }
}

/// Presents the bank-transfer sheet via the host's
/// [PaymentSheetPresenter].
struct BankTransferPaymentHandler: PaymentHandler {
    let kind: PaymentMethodKind = .bankTransfer
    let statusWatcher: (String) -> AsyncStream<PaymentStatusUpdate>

    func handle(_ result: CheckoutResult, presenter: PaymentSheetPresenter) async throws {
        guard case .bankTransfer(let ref, _) = result else {
            throw PaymentError.http(0, "BankTransferPaymentHandler received non-bank result")
        }
        presenter.presentBankTransferSheet(result, statusStream: statusWatcher(ref))
    }
}
