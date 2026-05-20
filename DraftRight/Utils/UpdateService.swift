import Foundation
import AppKit

/// One platform's entry in the `platforms` map of `/updates/latest`.
struct PlatformRelease: Codable {
    let version: String
    let url: String
    let notes: String?
    let required: Bool?
}

struct UpdateInfo: Codable {
    // Legacy top-level fields (kept for older clients). The top-level
    // `version` is now the highest version across all platforms, so it's
    // still safe to compare against — but always download via the
    // per-platform entry below when it's present.
    let version: String
    let mac_url: String
    let windows_url: String
    let linux_url: String
    let release_notes: String
    let required: Bool
    // Per-platform expansion. Prefer this when present.
    let platforms: [String: PlatformRelease]?

    /// Resolve the release applicable to `platform` ("mac"), preferring the
    /// `platforms` map and falling back to the legacy top-level fields.
    func resolved(for platform: String) -> ResolvedUpdate {
        if let p = platforms?[platform] {
            return ResolvedUpdate(version: p.version, url: p.url,
                                  notes: p.notes ?? release_notes,
                                  required: p.required ?? required)
        }
        let legacyURL: String
        switch platform {
        case "windows": legacyURL = windows_url
        case "linux": legacyURL = linux_url
        default: legacyURL = mac_url
        }
        return ResolvedUpdate(version: version, url: legacyURL,
                              notes: release_notes, required: required)
    }
}

/// A concrete update target the UI can act on.
struct ResolvedUpdate: Equatable {
    let version: String
    let url: String
    let notes: String
    let required: Bool
}

@MainActor
final class UpdateService: ObservableObject {
    private let currentVersion: String
    private let backendUrl: String
    private var lastCheck: Date = .distantPast
    private let checkInterval: TimeInterval = 86400 // 24 hours

    /// Newest release applicable to this Mac (strictly newer + non-empty
    /// URL), or nil if up to date / not yet checked. Drives the menu-bar
    /// "Update available" item and the Settings link.
    @Published private(set) var availableUpdate: ResolvedUpdate?

    /// True once `availableUpdate`'s DMG has been silently pre-downloaded and
    /// is sitting on disk — "install" is then instant (no download, no hang).
    @Published private(set) var updateStaged: Bool = false

    private var stagedDMGPath: String?
    private var stagedVersion: String?

    init(currentVersion: String, backendUrl: String) {
        self.currentVersion = currentVersion
        self.backendUrl = backendUrl.strippingTrailingSlash
    }

    func checkIfNeeded() async {
        guard Date().timeIntervalSince(lastCheck) > checkInterval else { return }
        lastCheck = Date()
        // Silent path: refreshAvailableUpdate kicks off background staging.
        // promptRestart() runs only when staging completes (see stageUpdate).
        _ = await refreshAvailableUpdate()
        if let update = availableUpdate, updateStaged, stagedVersion == update.version {
            await MainActor.run { promptRestart(update) }
        }
    }

    /// Manual check — skips the 24h throttle. Called from Settings > Check for Updates.
    /// If an update is already staged → restart prompt.
    /// If an update applies but isn't staged yet → keep staging silently and tell
    /// the user we're downloading. The next stageUpdate completion will prompt.
    /// If no update → confirm "you're current".
    func checkNow() async {
        lastCheck = Date()
        guard let update = await refreshAvailableUpdate() else {
            let alert = NSAlert()
            alert.messageText = "No Updates Available"
            alert.informativeText = "You're running the latest version (v\(currentVersion))."
            alert.alertStyle = .informational
            alert.runModal()
            return
        }
        if updateStaged, stagedVersion == update.version {
            await MainActor.run { promptRestart(update) }
        } else {
            await MainActor.run {
                let alert = NSAlert()
                alert.messageText = "Downloading DraftRight v\(update.version)"
                alert.informativeText = "We'll let you know when it's ready to install. You can keep working."
                alert.alertStyle = .informational
                alert.runModal()
            }
        }
    }

    /// Fetch `/updates/latest`, recompute `availableUpdate`, return it (or nil).
    /// Kicks off a silent background download of the new DMG when one applies.
    @discardableResult
    func refreshAvailableUpdate() async -> ResolvedUpdate? {
        guard let info = await fetchLatestVersion() else {
            setAvailable(nil)
            return nil
        }
        let candidate = info.resolved(for: "mac")
        let applicable = (isNewer(remote: candidate.version, local: currentVersion)
                          && !candidate.url.isEmpty) ? candidate : nil
        setAvailable(applicable)
        if let applicable, !(updateStaged && stagedVersion == applicable.version) {
            Task { await stageUpdate(applicable) }
        }
        return applicable
    }

    private func setAvailable(_ update: ResolvedUpdate?) {
        if update?.version != availableUpdate?.version {
            // Different (or no) update — any staged DMG is stale.
            updateStaged = false
            stagedDMGPath = nil
            stagedVersion = nil
        }
        availableUpdate = update
    }

    /// Silently downloads the update's DMG to a temp file in the background
    /// (retries on stalls/failures, no progress window). When it lands,
    /// `updateStaged` flips true and "install" becomes instant.
    private func stageUpdate(_ update: ResolvedUpdate) async {
        guard !update.url.isEmpty, let url = URL(string: update.url) else { return }
        if updateStaged, stagedVersion == update.version { return }
        let dmgPath = NSTemporaryDirectory() + "DraftRight-\(update.version).dmg"
        let dmgURL = URL(fileURLWithPath: dmgPath)
        for attempt in 1...3 {
            do {
                try? FileManager.default.removeItem(at: dmgURL)
                let tempURL = try await downloadWithProgress(from: url, onProgress: { _ in })
                try FileManager.default.moveItem(at: tempURL, to: dmgURL)
                // The available update may have changed while downloading.
                guard availableUpdate?.version == update.version else {
                    try? FileManager.default.removeItem(at: dmgURL)
                    return
                }
                stagedDMGPath = dmgPath
                stagedVersion = update.version
                updateStaged = true
                DRLogger.log("Update \(update.version) staged at \(dmgPath)", category: .app)
                // Silent-install UX: now that the DMG is on disk, ask the
                // user only the question that matters — restart now or
                // later. The download phase that used to require attention
                // has happened in the background.
                await MainActor.run { promptRestart(update) }
                return
            } catch {
                DRLogger.warn("Update staging attempt \(attempt)/3 failed: \(error.localizedDescription)", category: .app)
                try? FileManager.default.removeItem(at: dmgURL)
                if attempt < 3 {
                    try? await Task.sleep(nanoseconds: UInt64(attempt) * 5_000_000_000)
                }
            }
        }
    }

    /// Install the given update. If its DMG was already staged in the
    /// background, mounts and installs it immediately (no download wait);
    /// otherwise falls back to download-then-install with a progress window.
    /// Public entry point for the menu-bar item / Settings link.
    func startInstall(_ update: ResolvedUpdate) {
        guard !update.url.isEmpty else { return }
        if updateStaged, stagedVersion == update.version, let path = stagedDMGPath,
           FileManager.default.fileExists(atPath: path) {
            Task { await installStagedDMG(at: path, version: update.version) }
        } else {
            Task { await downloadAndInstall(url: update.url, version: update.version) }
        }
    }

    /// Install from a DMG that's already on disk. Silent: no progress UI —
    /// the staged DMG sits on local disk so mount + copy is sub-second. Only
    /// the user-visible restart event matters; everything else is plumbing.
    private func installStagedDMG(at dmgPath: String, version: String) async {
        do {
            try installDMG(at: dmgPath)
            DRLogger.log("Update \(version) installed silently; relaunching", category: .app)
            relaunch()
        } catch {
            DRLogger.error("Update install failed: \(error.localizedDescription)", category: .app)
            let alert = NSAlert()
            alert.messageText = "Update Failed"
            alert.informativeText = "Could not install the update: \(error.localizedDescription)"
            alert.alertStyle = .critical
            alert.runModal()
        }
    }

    private func fetchLatestVersion() async -> UpdateInfo? {
        guard let url = URL(string: "\(backendUrl)/updates/latest") else { return nil }
        do {
            var request = URLRequest(url: url)
            request.timeoutInterval = 10
            let (data, _) = try await URLSession.shared.data(for: request)
            return try JSONDecoder().decode(UpdateInfo.self, from: data)
        } catch {
            DRLogger.warn("Update check failed: \(error.localizedDescription)", category: .app)
            return nil
        }
    }

    private func isNewer(remote: String, local: String) -> Bool {
        let r = remote.split(separator: ".").compactMap { Int($0) }
        let l = local.split(separator: ".").compactMap { Int($0) }
        for i in 0..<max(r.count, l.count) {
            let rv = i < r.count ? r[i] : 0
            let lv = i < l.count ? l[i] : 0
            if rv > lv { return true }
            if rv < lv { return false }
        }
        return false
    }

    /// Single user-facing prompt for the silent-update flow. Shown only when
    /// the DMG is already staged on disk. Restart Now → installStagedDMG +
    /// relaunch (sub-second). Later → no-op; the user gets re-prompted on
    /// the next checkIfNeeded tick or on Settings → Check for Updates.
    private func promptRestart(_ update: ResolvedUpdate) {
        // Don't double-prompt if a prompt is already in front.
        guard !isPrompting else { return }
        isPrompting = true
        defer { isPrompting = false }

        let alert = NSAlert()
        alert.messageText = "Restart to apply DraftRight v\(update.version)"
        alert.informativeText = update.notes.isEmpty
            ? "The update is downloaded and ready. Restart now to apply."
            : "\(update.notes)\n\nThe update is downloaded and ready. Restart now to apply."
        alert.alertStyle = .informational
        alert.addButton(withTitle: "Restart Now")
        if !update.required {
            alert.addButton(withTitle: "Later")
        }

        let response = alert.runModal()
        if response == .alertFirstButtonReturn {
            startInstall(update)
        }
    }

    /// One-shot guard so the silent staging path doesn't pop a second prompt
    /// while one is still on screen (e.g. checkIfNeeded fires while the user
    /// is staring at the dialog from a previous tick).
    private var isPrompting = false

    private func downloadAndInstall(url: String, version: String) async {
        guard let downloadURL = URL(string: url) else { return }

        DRLogger.log("Downloading update from \(url)", category: .app)

        // Show progress window
        let progressWindow = UpdateProgressWindow(version: version)
        progressWindow.show()

        do {
            let dmgPath = NSTemporaryDirectory() + "DraftRight-\(version).dmg"
            let dmgURL = URL(fileURLWithPath: dmgPath)
            try? FileManager.default.removeItem(at: dmgURL)

            // Download with progress tracking
            let tempURL = try await downloadWithProgress(from: downloadURL) { fraction in
                Task { @MainActor in progressWindow.updateProgress(fraction) }
            }
            try FileManager.default.moveItem(at: tempURL, to: dmgURL)

            progressWindow.updateStatus("Installing...")
            progressWindow.setIndeterminate()

            try installDMG(at: dmgPath)

            progressWindow.close()
            relaunch()
        } catch {
            progressWindow.close()
            DRLogger.error("Update failed: \(error.localizedDescription)", category: .app)
            let alert = NSAlert()
            alert.messageText = "Update Failed"
            alert.informativeText = "Could not install the update: \(error.localizedDescription)"
            alert.alertStyle = .critical
            alert.runModal()
        }
    }

    /// Mount the DMG at `dmgPath`, copy the .app inside it to /Applications
    /// (replacing the current one), unmount, and delete the DMG.
    private func installDMG(at dmgPath: String) throws {
        let mountPoint = try mountDMG(at: dmgPath)
        defer { unmountDMG(mountPoint) }

        let contents = try FileManager.default.contentsOfDirectory(atPath: mountPoint)
        guard let appName = contents.first(where: { $0.hasSuffix(".app") }) else {
            throw NSError(domain: "UpdateService", code: 4,
                          userInfo: [NSLocalizedDescriptionKey: "No .app found in the disk image"])
        }
        let source = "\(mountPoint)/\(appName)"
        let dest = "/Applications/DraftRight.app"
        try? FileManager.default.removeItem(atPath: dest)
        try FileManager.default.copyItem(atPath: source, toPath: dest)
        try? FileManager.default.removeItem(atPath: dmgPath)
    }

    private func downloadWithProgress(from url: URL,
                                      onProgress: @escaping @Sendable (Double) -> Void) async throws -> URL {
        try await withCheckedThrowingContinuation { continuation in
            let delegate = DownloadDelegate(
                onProgress: onProgress,
                onComplete: { tempURL, error in
                    if let error {
                        continuation.resume(throwing: error)
                    } else if let tempURL {
                        continuation.resume(returning: tempURL)
                    } else {
                        continuation.resume(throwing: NSError(domain: "UpdateService", code: 3,
                            userInfo: [NSLocalizedDescriptionKey: "Download failed"]))
                    }
                }
            )
            // Ephemeral config so we never serve a cached prior 404/HTML body
            // (we hit this exact bug today: Caddy 28KB HTML was cached and
            // re-served instead of the fresh DMG). Explicit long timeouts
            // because the default request timeout is wedging at ~30s on some
            // connections — much longer than the 1s real download time.
            let config = URLSessionConfiguration.ephemeral
            config.timeoutIntervalForRequest = 120
            config.timeoutIntervalForResource = 600
            config.requestCachePolicy = .reloadIgnoringLocalAndRemoteCacheData
            // Background queue for delegate callbacks — main is busy showing
            // the progress sheet and was occasionally starving the timer.
            let opQueue = OperationQueue()
            opQueue.maxConcurrentOperationCount = 1
            let session = URLSession(configuration: config, delegate: delegate, delegateQueue: opQueue)
            var request = URLRequest(url: url)
            request.cachePolicy = .reloadIgnoringLocalAndRemoteCacheData
            session.downloadTask(with: request).resume()
            DRLogger.log("Update download started: \(url.absoluteString)", category: .app)
        }
    }

    private func mountDMG(at path: String) throws -> String {
        let process = Process()
        process.executableURL = URL(fileURLWithPath: "/usr/bin/hdiutil")
        process.arguments = ["attach", path, "-nobrowse", "-readonly", "-mountrandom", "/tmp"]

        let pipe = Pipe()
        process.standardOutput = pipe
        try process.run()
        process.waitUntilExit()

        let output = String(data: pipe.fileHandleForReading.readDataToEndOfFile(), encoding: .utf8) ?? ""
        let lines = output.split(separator: "\n")
        guard let lastLine = lines.last else {
            throw NSError(domain: "UpdateService", code: 1, userInfo: [NSLocalizedDescriptionKey: "Failed to mount DMG"])
        }
        let parts = lastLine.split(separator: "\t")
        guard let mountPoint = parts.last?.trimmingCharacters(in: .whitespaces) else {
            throw NSError(domain: "UpdateService", code: 2, userInfo: [NSLocalizedDescriptionKey: "Could not parse mount point"])
        }
        return mountPoint
    }

    private func unmountDMG(_ mountPoint: String) {
        let process = Process()
        process.executableURL = URL(fileURLWithPath: "/usr/bin/hdiutil")
        process.arguments = ["detach", mountPoint, "-quiet"]
        try? process.run()
        process.waitUntilExit()
    }

    private func relaunch() {
        let appPath = "/Applications/DraftRight.app"
        let task = Process()
        task.executableURL = URL(fileURLWithPath: "/usr/bin/open")
        task.arguments = ["-n", appPath]
        try? task.run()

        DispatchQueue.main.asyncAfter(deadline: .now() + 0.5) {
            NSApp.terminate(nil)
        }
    }
}

// MARK: - Download delegate with progress tracking

private final class DownloadDelegate: NSObject, URLSessionDownloadDelegate {
    let onProgress: (Double) -> Void
    let onComplete: (URL?, Error?) -> Void

    init(onProgress: @escaping (Double) -> Void, onComplete: @escaping (URL?, Error?) -> Void) {
        self.onProgress = onProgress
        self.onComplete = onComplete
    }

    func urlSession(_ session: URLSession, downloadTask: URLSessionDownloadTask, didFinishDownloadingTo location: URL) {
        // Copy to a stable temp location before the session cleans up
        let dest = URL(fileURLWithPath: NSTemporaryDirectory() + "draftright-download-\(UUID().uuidString).tmp")
        try? FileManager.default.copyItem(at: location, to: dest)
        onComplete(dest, nil)
        session.finishTasksAndInvalidate()
    }

    func urlSession(_ session: URLSession, task: URLSessionTask, didCompleteWithError error: Error?) {
        if let error {
            onComplete(nil, error)
            session.finishTasksAndInvalidate()
        }
    }

    func urlSession(_ session: URLSession, downloadTask: URLSessionDownloadTask,
                    didWriteData bytesWritten: Int64, totalBytesWritten: Int64, totalBytesExpectedToWrite: Int64) {
        guard totalBytesExpectedToWrite > 0 else { return }
        let fraction = Double(totalBytesWritten) / Double(totalBytesExpectedToWrite)
        onProgress(fraction)
    }
}

// MARK: - Progress window

@MainActor
final class UpdateProgressWindow {
    private var window: NSWindow?
    private let progressBar: NSProgressIndicator
    private let statusLabel: NSTextField
    private let percentLabel: NSTextField

    init(version: String) {
        // Progress bar
        progressBar = NSProgressIndicator()
        progressBar.style = .bar
        progressBar.isIndeterminate = false
        progressBar.minValue = 0
        progressBar.maxValue = 1
        progressBar.doubleValue = 0
        progressBar.frame = NSRect(x: 20, y: 50, width: 310, height: 20)

        // Status label
        statusLabel = NSTextField(labelWithString: "Downloading DraftRight v\(version)...")
        statusLabel.font = .systemFont(ofSize: 13, weight: .medium)
        statusLabel.frame = NSRect(x: 20, y: 85, width: 310, height: 20)

        // Percent label
        percentLabel = NSTextField(labelWithString: "0%")
        percentLabel.font = .monospacedDigitSystemFont(ofSize: 12, weight: .regular)
        percentLabel.textColor = .secondaryLabelColor
        percentLabel.alignment = .center
        percentLabel.frame = NSRect(x: 20, y: 25, width: 310, height: 16)
    }

    func show() {
        let contentView = NSView(frame: NSRect(x: 0, y: 0, width: 350, height: 120))
        contentView.addSubview(statusLabel)
        contentView.addSubview(progressBar)
        contentView.addSubview(percentLabel)

        let win = NSWindow(
            contentRect: NSRect(x: 0, y: 0, width: 350, height: 120),
            styleMask: [.titled],
            backing: .buffered,
            defer: false
        )
        win.title = "Updating DraftRight"
        win.contentView = contentView
        win.center()
        win.isReleasedWhenClosed = false
        win.level = .floating
        win.makeKeyAndOrderFront(nil)
        NSApp.activate(ignoringOtherApps: true)
        self.window = win
    }

    func updateProgress(_ fraction: Double) {
        progressBar.doubleValue = fraction
        percentLabel.stringValue = "\(Int(fraction * 100))%"
    }

    func updateStatus(_ text: String) {
        statusLabel.stringValue = text
    }

    func setIndeterminate() {
        progressBar.isIndeterminate = true
        progressBar.startAnimation(nil)
        percentLabel.stringValue = ""
    }

    func close() {
        window?.orderOut(nil)
        window = nil
    }
}
