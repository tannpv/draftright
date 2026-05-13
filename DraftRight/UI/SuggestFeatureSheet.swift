import SwiftUI
import AppKit

private let feedbackPlatforms: [(value: String, label: String)] = [
    ("playground", "Playground (web)"),
    ("mobile",     "Mobile (iOS / Android)"),
    ("windows",    "Windows"),
    ("mac",        "macOS"),
    ("linux",      "Linux"),
]
private let feedbackBoardURL = URL(string: "https://draftright.info/feedback")!

/// "Suggest a feature" form — title + target-platform picker + details.
/// Opened from the menu bar and the Advanced settings tab.
struct SuggestFeatureSheet: View {
    @EnvironmentObject var appModel: AppModel
    @Environment(\.dismiss) private var dismiss

    @State private var title = ""
    @State private var platform = "mac"
    @State private var details = ""
    @State private var email = ""
    @State private var isSubmitting = false
    @State private var errorMessage: String? = nil

    private var trimmedTitle: String {
        title.trimmingCharacters(in: .whitespacesAndNewlines)
    }

    private var trimmedDetails: String {
        details.trimmingCharacters(in: .whitespacesAndNewlines)
    }

    private var canSubmit: Bool {
        !isSubmitting && !trimmedTitle.isEmpty && !trimmedDetails.isEmpty
    }

    var body: some View {
        VStack(alignment: .leading, spacing: 16) {
            header
            titleField
            platformPicker
            detailsField
            if !appModel.isLoggedIn {
                emailField
            }
            if let err = errorMessage {
                errorBanner(err)
            }
            footer
        }
        .padding(20)
        .frame(width: 460)
        .frame(minHeight: 400)
    }

    // MARK: - Sections

    private var header: some View {
        HStack {
            Text("Suggest a feature")
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

    private var titleField: some View {
        VStack(alignment: .leading, spacing: 4) {
            Text("What should we build?")
                .font(.headline)
            TextField("One line summary", text: $title)
                .textFieldStyle(.roundedBorder)
        }
    }

    private var platformPicker: some View {
        VStack(alignment: .leading, spacing: 4) {
            Text("For platform")
                .font(.headline)
            Picker("", selection: $platform) {
                ForEach(feedbackPlatforms, id: \.value) {
                    Text($0.label).tag($0.value)
                }
            }
            .labelsHidden()
            .pickerStyle(.menu)
        }
    }

    private var detailsField: some View {
        VStack(alignment: .leading, spacing: 4) {
            Text("Details")
                .font(.headline)
            TextEditor(text: $details)
                .font(.body)
                .frame(minHeight: 100)
                .padding(6)
                .background(Color(nsColor: .textBackgroundColor))
                .cornerRadius(6)
                .overlay(
                    RoundedRectangle(cornerRadius: 6)
                        .stroke(Color.secondary.opacity(0.3), lineWidth: 1)
                )
        }
    }

    private var emailField: some View {
        VStack(alignment: .leading, spacing: 4) {
            Text("Email (optional)")
                .font(.headline)
            TextField("you@example.com", text: $email)
                .textFieldStyle(.roundedBorder)
            Text("So we can follow up with you.")
                .font(.caption)
                .foregroundColor(.secondary)
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
            Button("See all requests →") {
                NSWorkspace.shared.open(feedbackBoardURL)
            }
            .buttonStyle(.link)
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
                    Text(isSubmitting ? "Submitting…" : "Submit request")
                }
            }
            .keyboardShortcut(.defaultAction)
            .disabled(!canSubmit)
        }
    }

    // MARK: - Submit

    private func submit() async {
        errorMessage = nil
        isSubmitting = true
        defer { isSubmitting = false }

        let token = appModel.accessToken.isEmpty ? nil : appModel.accessToken
        let emailArg: String? = {
            let trimmed = email.trimmingCharacters(in: .whitespacesAndNewlines)
            return trimmed.isEmpty ? nil : trimmed
        }()

        do {
            _ = try await FeedbackService.submitFeatureRequest(
                title: trimmedTitle,
                targetPlatform: platform,
                description: trimmedDetails,
                userEmail: emailArg,
                authToken: token,
                backendUrl: appModel.backendUrl
            )
            dismiss()
            FeedbackPresenter.showThanksAlert()
        } catch {
            errorMessage = error.localizedDescription
        }
    }
}

/// Presents `SuggestFeatureSheet` in its own floating NSWindow.
/// The macOS app has `LSUIElement = true` so there's no main window to
/// attach a `.sheet()` to — we open a small floating window instead.
/// Matches `BugReportPresenter` exactly: `NSHostingController` + `NSWindow(contentViewController:)`
/// + `willCloseNotification` observer to reset the cached reference.
@MainActor
enum FeedbackPresenter {
    private static var openWindow: NSWindow?

    static func present(appModel: AppModel) {
        if let existing = openWindow {
            existing.makeKeyAndOrderFront(nil)
            NSApp.activate(ignoringOtherApps: true)
            return
        }

        let view = SuggestFeatureSheet()
            .environmentObject(appModel)

        let hosting = NSHostingController(rootView: view)
        let window = NSWindow(contentViewController: hosting)
        window.title = "Suggest a Feature"
        window.styleMask = [.titled, .closable]
        window.isReleasedWhenClosed = false
        window.setContentSize(NSSize(width: 460, height: 460))
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
                FeedbackPresenter.openWindow = nil
            }
        }
    }

    static func showThanksAlert() {
        let alert = NSAlert()
        alert.messageText = "Thanks!"
        alert.informativeText = "Your feature request was submitted."
        alert.alertStyle = .informational
        alert.addButton(withTitle: "OK")
        NSApp.activate(ignoringOtherApps: true)
        alert.runModal()
    }
}
