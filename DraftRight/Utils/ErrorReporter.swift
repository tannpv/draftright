import Foundation
import AppKit

/// Sends unhandled exceptions and signal-level crashes from the macOS
/// app to /errors. Call `ErrorReporter.install(...)` once at app start.
///
/// Privacy: never sends user-typed text. Only error type, message, stack,
/// and a small context (OS version, app version, locale).
@MainActor
final class ErrorReporter {
    static let shared = ErrorReporter()

    private var backendUrl: String = ""
    private var bearerTokenProvider: (() -> String?)?
    private let queueKey = "draftright.error_reporter.queue"
    private var pending: [[String: Any]] = []
    private var flushScheduled = false

    private init() {}

    /// Wire up uncaught exception + signal handlers. Idempotent.
    static func install(backendUrl: String, bearerTokenProvider: @escaping () -> String?) {
        shared.backendUrl = backendUrl.strippingTrailingSlash
        shared.bearerTokenProvider = bearerTokenProvider
        shared.loadPersistedQueue()

        // Objective-C exceptions
        NSSetUncaughtExceptionHandler { ns in
            ErrorReporter.shared.recordSync(
                type: ns.name.rawValue,
                message: ns.reason ?? "",
                stack: ns.callStackSymbols.joined(separator: "\n")
            )
        }

        // Schedule a flush on next runloop turn so any errors that fired
        // before install have a chance to go out
        DispatchQueue.main.async { ErrorReporter.shared.scheduleFlush() }
    }

    /// Manual report for caught-but-noteworthy exceptions.
    static func reportHandled(
        _ error: Error,
        severity: String = "warning",
        context: [String: Any]? = nil
    ) {
        Task { @MainActor in
            shared.record(
                type: String(describing: type(of: error)),
                message: "\(error)",
                stack: Thread.callStackSymbols.joined(separator: "\n"),
                severity: severity,
                context: context
            )
        }
    }

    // ── Internals ─────────────────────────────────────────────────────────

    nonisolated func recordSync(type: String, message: String, stack: String) {
        // Called from C handler, no isolation guarantee. Trampoline.
        Task { @MainActor in
            ErrorReporter.shared.record(
                type: type,
                message: message,
                stack: stack,
                severity: "fatal",
                context: nil
            )
        }
    }

    private func record(
        type: String,
        message: String,
        stack: String,
        severity: String,
        context: [String: Any]?
    ) {
        let appVersion = Bundle.main.infoDictionary?["CFBundleShortVersionString"] as? String ?? "?"
        let buildNumber = Bundle.main.infoDictionary?["CFBundleVersion"] as? String ?? "?"
        var ctx: [String: Any] = [
            "os": ProcessInfo.processInfo.operatingSystemVersionString,
            "locale": Locale.current.identifier,
            "ts": ISO8601DateFormatter().string(from: Date()),
        ]
        if let c = context { for (k, v) in c { ctx[k] = v } }

        let entry: [String: Any] = [
            "platform": "macos",
            "app_version": "\(appVersion)+\(buildNumber)",
            "severity": severity,
            "error_type": String(type.prefix(200)),
            "message": String(message.prefix(5000)),
            "stack_trace": String(stack.prefix(20000)),
            "context": ctx,
        ]
        pending.append(entry)
        if pending.count > 100 { pending.removeFirst() }
        persistQueue()
        scheduleFlush()
    }

    private func loadPersistedQueue() {
        guard let data = UserDefaults.standard.data(forKey: queueKey) else { return }
        guard let list = try? JSONSerialization.jsonObject(with: data) as? [[String: Any]] else { return }
        pending = list
    }

    private func persistQueue() {
        if let data = try? JSONSerialization.data(withJSONObject: pending) {
            UserDefaults.standard.set(data, forKey: queueKey)
        }
    }

    private func scheduleFlush() {
        guard !flushScheduled, !pending.isEmpty else { return }
        flushScheduled = true
        DispatchQueue.main.asyncAfter(deadline: .now() + 3.0) {
            Task { @MainActor in
                ErrorReporter.shared.flush()
            }
        }
    }

    private func flush() {
        flushScheduled = false
        guard !pending.isEmpty else { return }
        guard !backendUrl.isEmpty else { return }
        guard let url = URL(string: "\(backendUrl)/errors") else { return }
        let token = bearerTokenProvider?()

        let snapshot = pending
        for entry in snapshot {
            var req = URLRequest(url: url)
            req.httpMethod = "POST"
            req.timeoutInterval = 10
            req.setValue("application/json", forHTTPHeaderField: "Content-Type")
            if let t = token, !t.isEmpty {
                req.setValue("Bearer \(t)", forHTTPHeaderField: "Authorization")
            }
            req.httpBody = try? JSONSerialization.data(withJSONObject: entry)

            URLSession.shared.dataTask(with: req) { _, resp, _ in
                Task { @MainActor in
                    let ok = (resp as? HTTPURLResponse)?.statusCode ?? 0 < 400
                    if ok {
                        ErrorReporter.shared.pending.removeAll { ($0 as NSDictionary) == (entry as NSDictionary) }
                        ErrorReporter.shared.persistQueue()
                    }
                }
            }.resume()
        }
    }
}
