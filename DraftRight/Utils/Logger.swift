import Foundation

/// Centralized file logger for DraftRight.
/// Logs to ~/Library/Logs/DraftRight/draftright.log with timestamps and categories.
enum DRLogger {
    /// Toggle logging on/off. Persisted in UserDefaults.
    static var isEnabled: Bool {
        get { UserDefaults.standard.bool(forKey: "draftright.loggingEnabled") }
        set { UserDefaults.standard.set(newValue, forKey: "draftright.loggingEnabled") }
    }

    private static let logDir: URL = {
        let dir = FileManager.default.homeDirectoryForCurrentUser
            .appendingPathComponent("Library/Logs/DraftRight")
        try? FileManager.default.createDirectory(at: dir, withIntermediateDirectories: true)
        return dir
    }()

    private static let logFile: URL = logDir.appendingPathComponent("draftright.log")

    private static let dateFormatter: DateFormatter = {
        let f = DateFormatter()
        f.dateFormat = "yyyy-MM-dd HH:mm:ss.SSS"
        return f
    }()

    enum Category: String {
        case app = "APP"
        case auth = "AUTH"
        case ax = "AX"
        case monitor = "MONITOR"
        case panel = "PANEL"
        case api = "API"
        case settings = "SETTINGS"
    }

    /// Log severity. Lines are tagged `[LEVEL]` so failures are skimmable and
    /// greppable (`grep "\[ERROR\]"`) instead of buried in same-level category
    /// chatter. `.warn` = something degraded but handled; `.error` = an
    /// operation failed.
    enum Level: String {
        case info = "INFO"
        case warn = "WARN"
        case error = "ERROR"
        // Threshold-only — used as `minLevel` to silence everything; never
        // passed as a line's level.
        case off = "OFF"

        /// Severity ordering for threshold comparisons.
        var rank: Int {
            switch self {
            case .info: return 0
            case .warn: return 1
            case .error: return 2
            case .off: return 3
            }
        }
    }

    /// Minimum severity to write, set remotely by the admin portal and pushed
    /// to clients via `/health`'s `client_log_level`. The master verbosity
    /// control: `.info` logs everything (default), `.off` is the absolute
    /// kill-switch (silences even errors, for privacy/compliance).
    static var minLevel: Level = .info

    static func log(_ message: String, category: Category = .app, level: Level = .info) {
        // Remote admin threshold (master): drop anything below it; .off drops all.
        guard level.rank >= minLevel.rank else { return }
        // WARN/ERROR are always recorded, even when the user has logging
        // toggled off — a bug report shouldn't be blank for the lines that
        // matter most. Only routine INFO chatter honors the off-switch.
        guard isEnabled || level != .info else { return }

        let timestamp = dateFormatter.string(from: Date())
        // Pad the level so the [CATEGORY] column stays aligned across lines.
        let levelTag = level.rawValue.padding(toLength: 5, withPad: " ", startingAt: 0)
        let line = "[\(timestamp)] [\(levelTag)] [\(category.rawValue)] \(message)\n"

        if let data = line.data(using: .utf8) {
            if let handle = try? FileHandle(forWritingTo: logFile) {
                handle.seekToEndOfFile()
                handle.write(data)
                handle.closeFile()
            } else {
                try? data.write(to: logFile)
            }
        }

        #if DEBUG
        print(line, terminator: "")
        #endif
    }

    /// Log a handled-but-degraded condition at `.warn`.
    static func warn(_ message: String, category: Category = .app) {
        log(message, category: category, level: .warn)
    }

    /// Log a failed operation at `.error`.
    static func error(_ message: String, category: Category = .app) {
        log(message, category: category, level: .error)
    }

    /// Applies the backend's `client_log_level` ("off" | "errors" | "warnings"
    /// | "info") as the `minLevel` threshold. Unknown/empty → full logging.
    /// No-op when unchanged so the ~30s health poll doesn't spam; only a
    /// genuine change is announced.
    static func setMinLevelFromServer(_ value: String?) {
        let newLevel: Level
        switch value?.trimmingCharacters(in: .whitespaces).lowercased() {
        case "off": newLevel = .off
        case "errors", "error": newLevel = .error
        case "warnings", "warning", "warn": newLevel = .warn
        default: newLevel = .info
        }
        if newLevel == minLevel { return }

        let old = minLevel
        let msg = "Client log level changed: \(old.rawValue) -> \(newLevel.rawValue) (server '\(value ?? "")')"
        // When narrowing (e.g. → off) record under the OLD threshold first so
        // the announcement isn't dropped; when widening, set then record.
        if newLevel.rank > old.rank {
            log(msg, category: .app, level: .warn)
            minLevel = newLevel
        } else {
            minLevel = newLevel
            log(msg, category: .app, level: .warn)
        }
    }

    /// Returns the log file path for display in Settings
    static var logFilePath: String { logFile.path }

    /// Returns the last N lines of the log
    static func tail(lines: Int = 100) -> String {
        guard let content = try? String(contentsOf: logFile, encoding: .utf8) else { return "(no logs)" }
        let allLines = content.components(separatedBy: "\n")
        let lastLines = allLines.suffix(lines)
        return lastLines.joined(separator: "\n")
    }

    /// Clears the log file
    static func clear() {
        try? "".write(to: logFile, atomically: true, encoding: .utf8)
    }
}
