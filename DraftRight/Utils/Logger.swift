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
    }

    static func log(_ message: String, category: Category = .app, level: Level = .info) {
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
