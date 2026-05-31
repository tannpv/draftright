import Foundation

/// HTTP layer for the payment subsystem.  Stays separate from the
/// existing [BackendClient] so the rewrite + payment paths can evolve
/// independently; both share the same access token from [AppModel].
@MainActor
final class PaymentClient {
    private let session: URLSession
    private weak var appModel: AppModel?

    init(appModel: AppModel, session: URLSession = .shared) {
        self.appModel = appModel
        self.session = session
    }

    // MARK: - Plan catalog (unauthenticated)

    /// GET /plans → raw plan rows.  Used by [PaymentService] to pick
    /// the Pro tier id without leaking the list into the UI layer.
    func listPlans() async throws -> [[String: Any]] {
        let json = try await getJSONAny(path: "/plans", authed: false)
        if let arr = json as? [[String: Any]] { return arr }
        return []
    }

    // MARK: - Methods discovery

    /// GET /payment/methods → list of [PaymentMethodKind] (wire-name
    /// values unknown to this client are filtered out).
    func listPaymentMethods() async throws -> [PaymentMethodKind] {
        let json = try await getJSON(path: "/payment/methods", authed: false)
        guard let raw = json["methods"] as? [String] else { return [] }
        return raw.compactMap(PaymentMethodKind.fromWire)
    }

    // MARK: - Checkout

    /// POST /payment/checkout → typed [CheckoutResult].
    func createCheckout(planId: String, method: PaymentMethodKind) async throws -> CheckoutResult {
        let body: [String: Any] = ["plan_id": planId, "method": method.wireName]
        let json = try await postJSON(path: "/payment/checkout", body: body, authed: true)
        return try CheckoutResult.decode(json)
    }

    // MARK: - Status

    /// GET /payment/status/:ref → one status snapshot.
    func getPaymentStatus(referenceCode: String) async throws -> PaymentStatusUpdate {
        let json = try await getJSON(path: "/payment/status/\(referenceCode)", authed: false)
        return PaymentStatusUpdate(json: json)
    }

    // MARK: - Customer portal

    /// GET /lemonsqueezy/portal → one-shot LS Customer Portal URL.
    /// Throws on Stripe-only subscriptions until backend offers a
    /// unified portal endpoint.
    func getCustomerPortalURL() async throws -> URL {
        let json = try await getJSON(path: "/lemonsqueezy/portal", authed: true)
        guard let s = json["url"] as? String, let url = URL(string: s) else {
            throw PaymentError.missingURL
        }
        return url
    }

    // MARK: - Subscription info

    /// GET /subscription → current plan + usage envelope (raw JSON;
    /// caller decides which keys to surface).
    func getSubscription() async throws -> [String: Any] {
        return try await getJSON(path: "/subscription", authed: true)
    }

    // MARK: - Internal HTTP plumbing

    private var baseURL: String {
        (appModel?.backendUrl ?? AppModel.defaultBackendUrl)
            .trimmingCharacters(in: ["/"])
    }

    private func getJSON(path: String, authed: Bool) async throws -> [String: Any] {
        let raw = try await getJSONAny(path: path, authed: authed)
        return (raw as? [String: Any]) ?? [:]
    }

    /// GET that returns whatever JSON shape the server emits.  Used
    /// for `/plans` (root array) and the Map-shaped endpoints.
    private func getJSONAny(path: String, authed: Bool) async throws -> Any {
        guard let url = URL(string: "\(baseURL)\(path)") else {
            throw PaymentError.http(0, "Bad URL: \(path)")
        }
        var request = URLRequest(url: url)
        request.httpMethod = "GET"
        request.setValue("application/json", forHTTPHeaderField: "Accept")
        if authed, let token = appModel?.accessToken, !token.isEmpty {
            request.setValue("Bearer \(token)", forHTTPHeaderField: "Authorization")
        }
        return try await send(request)
    }

    private func postJSON(path: String, body: [String: Any], authed: Bool) async throws -> [String: Any] {
        guard let url = URL(string: "\(baseURL)\(path)") else {
            throw PaymentError.http(0, "Bad URL: \(path)")
        }
        var request = URLRequest(url: url)
        request.httpMethod = "POST"
        request.setValue("application/json", forHTTPHeaderField: "Content-Type")
        request.setValue("application/json", forHTTPHeaderField: "Accept")
        if authed, let token = appModel?.accessToken, !token.isEmpty {
            request.setValue("Bearer \(token)", forHTTPHeaderField: "Authorization")
        }
        request.httpBody = try JSONSerialization.data(withJSONObject: body)
        let raw = try await send(request)
        return (raw as? [String: Any]) ?? [:]
    }

    private func send(_ request: URLRequest) async throws -> Any {
        let (data, response) = try await session.data(for: request)
        guard let http = response as? HTTPURLResponse else {
            throw PaymentError.http(0, "No HTTP response")
        }
        if http.statusCode >= 400 {
            let msg = String(data: data, encoding: .utf8) ?? "HTTP \(http.statusCode)"
            throw PaymentError.http(http.statusCode, msg)
        }
        if data.isEmpty { return [String: Any]() }
        return try JSONSerialization.jsonObject(with: data, options: [])
    }
}
