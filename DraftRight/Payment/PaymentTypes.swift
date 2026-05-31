import Foundation

/// Identity for one payment method advertised by `GET /payment/methods`.
///
/// Mirrors `PaymentMethod` enum on the backend and `PaymentMethodKind`
/// on the Flutter client (1:1, same `wireName`s).  Adding a new method
/// = add a case + extend `wireName` + add a descriptor.
enum PaymentMethodKind: String, CaseIterable {
    case lemonsqueezy
    case stripe
    case vietqr
    case bankTransfer
    case paypal

    var wireName: String {
        switch self {
        case .lemonsqueezy: return "lemonsqueezy"
        case .stripe:       return "stripe"
        case .vietqr:       return "vietqr"
        case .bankTransfer: return "bank_transfer"
        case .paypal:       return "paypal"
        }
    }

    /// Reverse mapping.  Returns nil for unknown strings so the catalog
    /// gracefully ignores methods this client doesn't yet implement
    /// (forward-compat with backend additions).
    static func fromWire(_ value: String) -> PaymentMethodKind? {
        PaymentMethodKind.allCases.first { $0.wireName == value }
    }
}

/// UI metadata for [PaymentMethodKind].  Display name + description
/// live client-side for now; one place to localize later.
struct PaymentMethodDescriptor {
    let kind: PaymentMethodKind
    let displayName: String
    let description: String
    let symbolName: String  // SF Symbol for the row icon

    static func forKind(_ kind: PaymentMethodKind) -> PaymentMethodDescriptor {
        switch kind {
        case .lemonsqueezy:
            return .init(
                kind: .lemonsqueezy,
                displayName: "Credit / Debit Card",
                description: "Visa, Mastercard, Apple Pay (via Lemon Squeezy)",
                symbolName: "creditcard"
            )
        case .stripe:
            return .init(
                kind: .stripe,
                displayName: "Stripe",
                description: "Credit card via Stripe",
                symbolName: "creditcard"
            )
        case .vietqr:
            return .init(
                kind: .vietqr,
                displayName: "VietQR (scan to pay)",
                description: "Scan with any Vietnamese banking app — auto-confirms",
                symbolName: "qrcode"
            )
        case .bankTransfer:
            return .init(
                kind: .bankTransfer,
                displayName: "Bank Transfer",
                description: "Manual transfer with reference code",
                symbolName: "building.columns"
            )
        case .paypal:
            return .init(
                kind: .paypal,
                displayName: "PayPal",
                description: "Pay with PayPal balance or card",
                symbolName: "wallet.pass"
            )
        }
    }
}

/// Typed view of the JSON returned by `POST /payment/checkout`.
///
/// Backend `CheckoutResult` is a union of three shapes (redirect /
/// qr / bank_info); model each as a case so UI dispatches on type,
/// not field presence.
enum CheckoutResult {
    case redirect(referenceCode: String, url: URL)
    case qr(referenceCode: String, imageURL: URL, bankInfo: BankInfo?)
    case bankTransfer(referenceCode: String, info: BankInfo)

    var referenceCode: String {
        switch self {
        case .redirect(let r, _),
             .qr(let r, _, _),
             .bankTransfer(let r, _):
            return r
        }
    }

    /// Decode from the controller's JSON envelope.  Field priority
    /// matches Flutter + backend: redirect → qr → bank.  Throws if
    /// none of the three fields are present.
    static func decode(_ json: [String: Any]) throws -> CheckoutResult {
        // reference_code lives either at the top level or inside `payment`.
        let ref: String = {
            if let payment = json["payment"] as? [String: Any],
               let s = payment["reference_code"] as? String { return s }
            if let s = json["reference_code"] as? String { return s }
            return ""
        }()

        if let redirect = json["redirect_url"] as? String,
           !redirect.isEmpty,
           let url = URL(string: redirect) {
            return .redirect(referenceCode: ref, url: url)
        }
        let bankInfo = (json["bank_info"] as? [String: Any]).flatMap(BankInfo.init(json:))
        if let qrData = json["qr_data"] as? String,
           !qrData.isEmpty,
           let url = URL(string: qrData) {
            return .qr(referenceCode: ref, imageURL: url, bankInfo: bankInfo)
        }
        if let info = bankInfo {
            return .bankTransfer(referenceCode: ref, info: info)
        }
        throw PaymentError.unknownCheckoutShape
    }
}

struct BankInfo {
    let bankName: String
    let accountNumber: String
    let accountName: String
    let amount: Double
    let currency: String
    let reference: String

    init?(json: [String: Any]) {
        guard let bankName = json["bank_name"] as? String,
              let accountNumber = json["account_number"] as? String,
              let accountName = json["account_name"] as? String,
              let reference = json["reference"] as? String else { return nil }
        self.bankName = bankName
        self.accountNumber = accountNumber
        self.accountName = accountName
        self.amount = (json["amount"] as? Double) ?? Double(json["amount"] as? Int ?? 0)
        self.currency = (json["currency"] as? String) ?? "VND"
        self.reference = reference
    }
}

/// Lifecycle of a single payment.  Mirrors backend `PaymentStatus`
/// enum + synthetic `notFound` from `/payment/status/:ref`.
enum PaymentStatus: String {
    case pending
    case completed
    case failed
    case expired
    case refunded
    case notFound = "not_found"
    case unknown

    static func fromWire(_ value: String) -> PaymentStatus {
        PaymentStatus(rawValue: value) ?? .unknown
    }

    var isTerminal: Bool {
        switch self {
        case .pending, .notFound, .unknown: return false
        case .completed, .failed, .expired, .refunded: return true
        }
    }

    var isSuccess: Bool { self == .completed }
}

/// One poll result returned by `/payment/status/:ref`.
struct PaymentStatusUpdate {
    let referenceCode: String
    let status: PaymentStatus
    let amount: Double?
    let currency: String?
    let planName: String?

    init(json: [String: Any]) {
        self.referenceCode = (json["reference_code"] as? String) ?? ""
        self.status = PaymentStatus.fromWire((json["status"] as? String) ?? "")
        self.amount = json["amount"] as? Double
        self.currency = json["currency"] as? String
        self.planName = json["plan_name"] as? String
    }
}

enum PaymentError: LocalizedError {
    case notConfigured
    case unknownCheckoutShape
    case noProPlan
    case missingURL
    case http(Int, String)

    var errorDescription: String? {
        switch self {
        case .notConfigured:       return "Backend has no payment method configured"
        case .unknownCheckoutShape:return "Backend returned an unrecognised checkout response"
        case .noProPlan:           return "Could not find a Pro plan in the catalog"
        case .missingURL:          return "Backend did not return a URL"
        case .http(let code, let msg): return "HTTP \(code): \(msg)"
        }
    }
}
