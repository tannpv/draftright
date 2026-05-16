import Foundation

/// Installs a per-user launchd agent (`com.draftright.app.v2.keepalive`) that
/// respawns DraftRight when it exits abnormally (crash, SIGKILL, OS-initiated
/// termination of menu-bar apps under memory pressure, etc.). A clean "Quit
/// DraftRight" from the menu exits with status 0 and is intentionally NOT
/// respawned.
///
/// We use launchd instead of `SMAppService.mainApp` because Login Items only
/// fire at user login; they don't supervise the running process. Once the app
/// dies mid-session, nothing brings it back until the next login. launchd's
/// `KeepAlive` directive solves that.
///
/// The agent's `RunAtLoad` flag mirrors the user's "Launch at Login"
/// preference: ON → app boots at login + survives crashes; OFF → agent
/// uninstalled entirely (no auto-launch, no respawn).
enum KeepAliveAgent {
    static let label = "com.draftright.app.v2.keepalive"

    /// Where launchd looks for per-user agents.
    static var plistURL: URL {
        let home = FileManager.default.homeDirectoryForCurrentUser
        return home
            .appendingPathComponent("Library/LaunchAgents")
            .appendingPathComponent("\(label).plist")
    }

    /// The absolute path of the currently-running app's main executable.
    /// Used so the installed plist tracks wherever the app is installed
    /// (typically `/Applications/DraftRight.app`, but users sometimes
    /// keep it elsewhere or run it from `~/Downloads/` during testing).
    static var executablePath: String? {
        Bundle.main.executablePath
    }

    /// True if the plist file exists at the expected location.
    static var isInstalled: Bool {
        FileManager.default.fileExists(atPath: plistURL.path)
    }

    /// Returns true if the existing plist's `ProgramArguments[0]` matches the
    /// current bundle executable path. False means the app has been moved
    /// since the plist was last written — caller should reinstall.
    static var isPathFresh: Bool {
        guard isInstalled,
              let current = executablePath,
              let data = try? Data(contentsOf: plistURL),
              let plist = try? PropertyListSerialization.propertyList(
                  from: data, options: [], format: nil) as? [String: Any],
              let args = plist["ProgramArguments"] as? [String],
              let installed = args.first
        else { return false }
        return installed == current
    }

    /// Install (or refresh) the launchd agent.
    /// - Parameter runAtLoad: launch the app automatically on user login.
    /// - Returns: true on success, false on any failure.
    @discardableResult
    static func install(runAtLoad: Bool) -> Bool {
        guard let exe = executablePath else {
            DRLogger.log("KeepAliveAgent: no executable path — aborting install", category: .app)
            return false
        }

        let plist: [String: Any] = [
            "Label": label,
            "ProgramArguments": [exe],
            "RunAtLoad": runAtLoad,
            // Respawn only on abnormal exit (SuccessfulExit=false means "if it
            // exited with non-zero status, restart it"). The throttle keeps
            // launchd from looping forever if startup itself is broken.
            "KeepAlive": ["SuccessfulExit": false],
            "ThrottleInterval": 10,
            "ProcessType": "Interactive",
            "LimitLoadToSessionType": "Aqua",
            "StandardOutPath": "/tmp/draftright-keepalive.log",
            "StandardErrorPath": "/tmp/draftright-keepalive.err",
        ]

        do {
            let dir = plistURL.deletingLastPathComponent()
            try FileManager.default.createDirectory(
                at: dir, withIntermediateDirectories: true)
            let data = try PropertyListSerialization.data(
                fromPropertyList: plist, format: .xml, options: 0)
            try data.write(to: plistURL, options: .atomic)
        } catch {
            DRLogger.log("KeepAliveAgent: write plist failed — \(error.localizedDescription)", category: .app)
            return false
        }

        // Re-load with launchctl: bootout first (ignoring errors when not
        // already loaded), then bootstrap the fresh plist into the user GUI
        // session. The `gui/<uid>` domain is what gives the agent access to
        // the user's Aqua session (necessary for an interactive app).
        let uid = getuid()
        _ = runLaunchctl(["bootout", "gui/\(uid)/\(label)"])
        guard runLaunchctl(["bootstrap", "gui/\(uid)", plistURL.path]) else {
            DRLogger.log("KeepAliveAgent: launchctl bootstrap failed", category: .app)
            return false
        }
        _ = runLaunchctl(["enable", "gui/\(uid)/\(label)"])
        return true
    }

    /// Remove the agent entirely (unloads from launchd + deletes the plist).
    @discardableResult
    static func uninstall() -> Bool {
        let uid = getuid()
        _ = runLaunchctl(["bootout", "gui/\(uid)/\(label)"])
        if isInstalled {
            do {
                try FileManager.default.removeItem(at: plistURL)
            } catch {
                DRLogger.log("KeepAliveAgent: remove plist failed — \(error.localizedDescription)", category: .app)
                return false
            }
        }
        return true
    }

    /// Sync the on-disk agent to the desired state. Idempotent — safe to call
    /// on every app launch.
    static func reconcile(desiredRunAtLoad: Bool) {
        if desiredRunAtLoad {
            // Reinstall if missing OR if the recorded path is stale (app
            // was moved). Otherwise leave alone — launchctl bootstrap is
            // not free.
            if !isInstalled || !isPathFresh {
                install(runAtLoad: true)
            }
        } else {
            if isInstalled {
                uninstall()
            }
        }
    }

    @discardableResult
    private static func runLaunchctl(_ args: [String]) -> Bool {
        let proc = Process()
        proc.executableURL = URL(fileURLWithPath: "/bin/launchctl")
        proc.arguments = args
        let pipe = Pipe()
        proc.standardError = pipe
        proc.standardOutput = pipe
        do {
            try proc.run()
            proc.waitUntilExit()
            return proc.terminationStatus == 0
        } catch {
            DRLogger.log("KeepAliveAgent: launchctl \(args.joined(separator: " ")) — \(error.localizedDescription)", category: .app)
            return false
        }
    }
}
