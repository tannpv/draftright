import SwiftUI
import AppKit
import UniformTypeIdentifiers

/// SwiftUI sheet that lets the user describe a bug, attach a screenshot
/// (browse / drag-drop / paste), and submit it to `/bug-reports`.
///
/// Mirrors the admin portal "Report a bug" UX: 3 ways to add an image,
/// inline preview, validation messages, success toast, error banner.
struct BugReportSheet: View {
    @EnvironmentObject var appModel: AppModel
    @Environment(\.dismiss) private var dismiss

    @State private var description: String = ""
    @State private var userEmail: String = ""
    @State private var screenshotImage: NSImage? = nil
    @State private var screenshotData: Data? = nil
    @State private var screenshotFilename: String = ""

    @State private var isSubmitting: Bool = false
    @State private var errorMessage: String? = nil
    @State private var isDragHovering: Bool = false

    /// Local NSEvent monitor so we can intercept Cmd+V while the sheet is up
    /// without hijacking the editor's normal paste behavior. Stored so the
    /// monitor can be removed in `onDisappear`.
    @State private var pasteMonitor: Any? = nil

    private var trimmedDescription: String {
        description.trimmingCharacters(in: .whitespacesAndNewlines)
    }

    private var canSubmit: Bool {
        !isSubmitting && trimmedDescription.count >= 10
    }

    var body: some View {
        VStack(alignment: .leading, spacing: 16) {
            header

            descriptionField

            if !appModel.isLoggedIn {
                emailField
            }

            screenshotSection

            if let err = errorMessage {
                errorBanner(err)
            }

            footer
        }
        .padding(20)
        .frame(width: 520)
        .frame(minHeight: 540)
        .onAppear { installPasteMonitor() }
        .onDisappear { removePasteMonitor() }
    }

    // MARK: - Sections

    private var header: some View {
        HStack {
            Text("Report a bug")
                .font(.title2)
                .bold()
            Spacer()
            Button {
                dismiss()
            } label: {
                Image(systemName: "xmark.circle.fill")
                    .font(.title2)
                    .foregroundColor(.secondary)
            }
            .buttonStyle(.plain)
            .help("Close")
            .keyboardShortcut(.cancelAction)
        }
    }

    private var descriptionField: some View {
        VStack(alignment: .leading, spacing: 4) {
            Text("Describe what happened")
                .font(.headline)
            TextEditor(text: $description)
                .font(.body)
                .frame(minHeight: 120)
                .padding(6)
                .background(Color(nsColor: .textBackgroundColor))
                .cornerRadius(6)
                .overlay(
                    RoundedRectangle(cornerRadius: 6)
                        .stroke(Color.secondary.opacity(0.3), lineWidth: 1)
                )
            HStack {
                Text("\(trimmedDescription.count) / 10 minimum")
                    .font(.caption)
                    .foregroundColor(trimmedDescription.count >= 10 ? .secondary : .orange)
                Spacer()
            }
        }
    }

    private var emailField: some View {
        VStack(alignment: .leading, spacing: 4) {
            Text("Email (optional)")
                .font(.headline)
            TextField("you@example.com", text: $userEmail)
                .textFieldStyle(.roundedBorder)
            Text("So we can reply if we need more info.")
                .font(.caption)
                .foregroundColor(.secondary)
        }
    }

    private var screenshotSection: some View {
        VStack(alignment: .leading, spacing: 6) {
            Text("Attach a screenshot (optional)")
                .font(.headline)
            if let image = screenshotImage {
                screenshotPreview(image: image)
            } else {
                screenshotDropZone
            }
            HStack(spacing: 12) {
                Button {
                    browseForScreenshot()
                } label: {
                    Label("Browse…", systemImage: "folder")
                }
                Button {
                    pasteScreenshotFromClipboard()
                } label: {
                    Label("Paste", systemImage: "doc.on.clipboard")
                }
                .help("Paste an image from the clipboard (Cmd+V)")
                if screenshotImage != nil {
                    Button(role: .destructive) {
                        clearScreenshot()
                    } label: {
                        Label("Remove", systemImage: "trash")
                    }
                }
                Spacer()
            }
            Text("Drag-drop, paste, or browse. PNG / JPEG, up to 5 MB.")
                .font(.caption)
                .foregroundColor(.secondary)
        }
    }

    private func screenshotPreview(image: NSImage) -> some View {
        Image(nsImage: image)
            .resizable()
            .scaledToFit()
            .frame(maxHeight: 180)
            .cornerRadius(6)
            .overlay(
                RoundedRectangle(cornerRadius: 6)
                    .stroke(Color.secondary.opacity(0.3), lineWidth: 1)
            )
    }

    private var screenshotDropZone: some View {
        VStack(spacing: 8) {
            Image(systemName: "photo.on.rectangle.angled")
                .font(.system(size: 28))
                .foregroundColor(.secondary)
            Text("Drag an image here, paste, or click Browse")
                .font(.callout)
                .foregroundColor(.secondary)
        }
        .frame(maxWidth: .infinity)
        .frame(height: 110)
        .background(
            RoundedRectangle(cornerRadius: 8)
                .fill(isDragHovering
                      ? Color.accentColor.opacity(0.08)
                      : Color.secondary.opacity(0.05))
        )
        .overlay(
            RoundedRectangle(cornerRadius: 8)
                .strokeBorder(
                    isDragHovering ? Color.accentColor : Color.secondary.opacity(0.3),
                    style: StrokeStyle(lineWidth: 1.5, dash: [6, 4])
                )
        )
        .onDrop(of: [.image, .fileURL], isTargeted: $isDragHovering) { providers in
            handleDrop(providers: providers)
        }
    }

    private func errorBanner(_ message: String) -> some View {
        HStack(alignment: .top, spacing: 8) {
            Image(systemName: "exclamationmark.triangle.fill")
                .foregroundColor(.red)
            Text(message)
                .font(.callout)
                .foregroundColor(.red)
                .fixedSize(horizontal: false, vertical: true)
            Spacer()
        }
        .padding(10)
        .background(Color.red.opacity(0.10))
        .cornerRadius(6)
    }

    private var footer: some View {
        HStack {
            Spacer()
            Button("Cancel") {
                dismiss()
            }
            .keyboardShortcut(.cancelAction)
            Button {
                Task { await submit() }
            } label: {
                HStack(spacing: 6) {
                    if isSubmitting {
                        ProgressView().scaleEffect(0.6)
                    }
                    Text(isSubmitting ? "Sending…" : "Send report")
                }
            }
            .keyboardShortcut(.defaultAction)
            .disabled(!canSubmit)
        }
    }

    // MARK: - Image input handlers

    private func browseForScreenshot() {
        let panel = NSOpenPanel()
        panel.canChooseFiles = true
        panel.canChooseDirectories = false
        panel.allowsMultipleSelection = false
        panel.allowedContentTypes = [.png, .jpeg]
        panel.title = "Choose screenshot"
        if panel.runModal() == .OK, let url = panel.url {
            loadScreenshot(fromFileURL: url)
        }
    }

    private func pasteScreenshotFromClipboard() {
        let pb = NSPasteboard.general
        // Prefer a real image first.
        if let image = NSImage(pasteboard: pb) {
            adoptImage(image, suggestedFilename: "pasted-\(timestampSuffix()).png")
            return
        }
        // Fallback: a file URL on the clipboard pointing at an image.
        if let urls = pb.readObjects(forClasses: [NSURL.self]) as? [URL],
           let url = urls.first {
            loadScreenshot(fromFileURL: url)
            return
        }
        errorMessage = "No image found on the clipboard."
    }

    private func handleDrop(providers: [NSItemProvider]) -> Bool {
        guard let provider = providers.first else { return false }

        // Try image data first (e.g. dragged from Photos / web).
        if provider.canLoadObject(ofClass: NSImage.self) {
            _ = provider.loadObject(ofClass: NSImage.self) { obj, _ in
                guard let image = obj as? NSImage else { return }
                Task { @MainActor in
                    adoptImage(image, suggestedFilename: "dropped-\(timestampSuffix()).png")
                }
            }
            return true
        }
        // Fallback: a dropped file URL.
        if provider.hasItemConformingToTypeIdentifier(UTType.fileURL.identifier) {
            provider.loadItem(forTypeIdentifier: UTType.fileURL.identifier, options: nil) { item, _ in
                var url: URL?
                if let data = item as? Data {
                    url = URL(dataRepresentation: data, relativeTo: nil)
                } else if let u = item as? URL {
                    url = u
                }
                guard let fileURL = url else { return }
                Task { @MainActor in
                    loadScreenshot(fromFileURL: fileURL)
                }
            }
            return true
        }
        return false
    }

    private func loadScreenshot(fromFileURL url: URL) {
        guard let data = try? Data(contentsOf: url) else {
            errorMessage = "Could not read \(url.lastPathComponent)."
            return
        }
        guard data.count <= 5 * 1024 * 1024 else {
            errorMessage = "Screenshot is larger than 5 MB."
            return
        }
        guard let image = NSImage(data: data) else {
            errorMessage = "Unsupported image format. Use PNG or JPEG."
            return
        }
        screenshotImage = image
        screenshotData = data
        screenshotFilename = url.lastPathComponent
        errorMessage = nil
    }

    private func adoptImage(_ image: NSImage, suggestedFilename: String) {
        // Re-encode to PNG so the upload size is predictable and the MIME
        // type matches the filename extension.
        guard let tiff = image.tiffRepresentation,
              let rep = NSBitmapImageRep(data: tiff),
              let png = rep.representation(using: .png, properties: [:]) else {
            errorMessage = "Could not encode image."
            return
        }
        guard png.count <= 5 * 1024 * 1024 else {
            errorMessage = "Screenshot is larger than 5 MB."
            return
        }
        screenshotImage = image
        screenshotData = png
        screenshotFilename = suggestedFilename
        errorMessage = nil
    }

    private func clearScreenshot() {
        screenshotImage = nil
        screenshotData = nil
        screenshotFilename = ""
    }

    // MARK: - Paste monitor (Cmd+V while sheet is open)

    private func installPasteMonitor() {
        // Local monitor lets us see Cmd+V while the sheet is key, but
        // doesn't suppress the event from reaching focused text editors.
        pasteMonitor = NSEvent.addLocalMonitorForEvents(matching: .keyDown) { event in
            // 9 == "v" key code on US layout (kVK_ANSI_V).
            if event.modifierFlags.contains(.command),
               event.keyCode == 9,
               !event.modifierFlags.contains(.option),
               !event.modifierFlags.contains(.control) {
                // Only intercept when the focused responder is NOT a text view
                // (let normal paste happen inside the description editor).
                if !isFocusInTextEditor() {
                    pasteScreenshotFromClipboard()
                    return nil
                }
            }
            return event
        }
    }

    private func removePasteMonitor() {
        if let mon = pasteMonitor {
            NSEvent.removeMonitor(mon)
            pasteMonitor = nil
        }
    }

    private func isFocusInTextEditor() -> Bool {
        guard let window = NSApp.keyWindow,
              let responder = window.firstResponder else { return false }
        return responder is NSTextView
    }

    // MARK: - Submit

    private func submit() async {
        errorMessage = nil
        isSubmitting = true
        defer { isSubmitting = false }

        let token = appModel.accessToken.isEmpty ? nil : appModel.accessToken
        let email: String? = {
            let trimmed = userEmail.trimmingCharacters(in: .whitespacesAndNewlines)
            return trimmed.isEmpty ? nil : trimmed
        }()
        let shot: (data: Data, filename: String)? = {
            if let data = screenshotData {
                let name = screenshotFilename.isEmpty
                    ? "screenshot-\(timestampSuffix()).png"
                    : screenshotFilename
                return (data, name)
            }
            return nil
        }()

        do {
            _ = try await BugReportService.submitReport(
                description: trimmedDescription,
                screenshot: shot,
                userEmail: email,
                authToken: token,
                backendUrl: appModel.backendUrl
            )
            dismiss()
            BugReportPresenter.showThanksAlert()
        } catch {
            errorMessage = error.localizedDescription
        }
    }

    private func timestampSuffix() -> String {
        let f = DateFormatter()
        f.dateFormat = "yyyyMMdd-HHmmss"
        return f.string(from: Date())
    }
}

/// Presents `BugReportSheet` in its own NSWindow modal-on-top-of-app.
/// The macOS app has `LSUIElement = true` so there's no main window to
/// attach a `.sheet()` to — we open a small floating window instead.
@MainActor
enum BugReportPresenter {
    private static var openWindow: NSWindow?

    static func present(appModel: AppModel) {
        if let existing = openWindow {
            existing.makeKeyAndOrderFront(nil)
            NSApp.activate(ignoringOtherApps: true)
            return
        }

        let view = BugReportSheet()
            .environmentObject(appModel)

        let hosting = NSHostingController(rootView: view)
        let window = NSWindow(contentViewController: hosting)
        window.title = "Report a bug"
        window.styleMask = [.titled, .closable]
        window.isReleasedWhenClosed = false
        window.setContentSize(NSSize(width: 520, height: 600))
        window.center()
        window.level = .floating
        window.makeKeyAndOrderFront(nil)
        NSApp.activate(ignoringOtherApps: true)
        openWindow = window

        // Clear the cached reference when the user closes the window so a
        // second invocation creates a fresh sheet (and resets all state).
        NotificationCenter.default.addObserver(
            forName: NSWindow.willCloseNotification,
            object: window,
            queue: .main
        ) { _ in
            Task { @MainActor in
                BugReportPresenter.openWindow = nil
            }
        }
    }

    static func showThanksAlert() {
        let alert = NSAlert()
        alert.messageText = "Thanks!"
        alert.informativeText = "Your bug report was sent. We'll look into it."
        alert.alertStyle = .informational
        alert.addButton(withTitle: "OK")
        NSApp.activate(ignoringOtherApps: true)
        alert.runModal()
    }
}
