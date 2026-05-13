# Feedback Client Submit Surfaces (Spec B) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a "Suggest a feature" submit form — title + target-platform dropdown + description — to every DraftRight client (web playground, macOS app, Windows app, Flutter iOS/Android, Linux app), each POSTing to the `POST /feedback` backend that already shipped in Spec A, plus a "See all requests →" link that opens `https://draftright.info/feedback`.

**Architecture:** Each client mirrors its existing bug-report submit UI but (a) sends **JSON** to `{backendBase}/feedback` (not multipart — feature requests have no screenshot), (b) adds a `title` field and a `target_platform` dropdown (`playground|mobile|windows|mac|linux`, pre-selected to that client's natural platform), (c) sends `kind:"feature"` and a `source` tag. Auth: attach the user's Bearer JWT when signed in (same token store each client already uses for bug reports). The backend validates everything; clients only do light client-side checks (non-empty title, non-empty description). The "See all requests →" link is a plain external-URL open — the board page (Spec C) doesn't exist yet but the link is harmless.

**Tech Stack:** Astro/React (`website/`), Swift/SwiftUI/AppKit (`DraftRight/`), C#/WinForms/WinUI3 (`DraftRightWindows/`), Dart/Flutter (`DraftRightMobile/`), Python/GTK4/libadwaita (`DraftRightLinux/`).

**`POST /feedback` contract (from Spec A):** JSON body `{ "kind": "feature", "title": "<3..80? no — 1..80 chars>", "target_platform": "playground|mobile|windows|mac|linux", "description": "<1..2000 chars>", "source": "<client tag>", "user_email": "<optional>" }`. Optional `Authorization: Bearer <jwt>` → backend stamps `user_id`. Returns `201 { "id": "<uuid>", "message": "Feature request received. Thanks!" }`. `400` on validation failure.

**Constants:**
- Public board URL (the "See all requests →" target): `https://draftright.info/feedback`
- `target_platform` defaults per client: web→`playground`, macOS→`mac`, Windows→`windows`, Flutter→`mobile`, Linux→`linux`
- `source` per client: web playground→`web`, macOS→`macos-app`, Windows→`windows-app`, Flutter→`ios-app`/`android-app` (auto), Linux→`linux-app`
- The dropdown options are always all five (`Playground / Mobile / Windows / macOS / Linux`); only the *pre-selection* differs by client.

**Tasks are independent per platform** — they touch disjoint codebases and can be executed in any order or in parallel. Do them as separate commits. Task 6 (docs) comes last.

---

### Task 1: Web playground — "Suggest a feature" widget

**Files:**
- Create: `website/src/components/SuggestFeatureWidget.tsx`
- Modify: `website/src/components/Playground.tsx` (render the widget near `<ReportBugWidget … />`)
- Reference (read for patterns, don't change): `website/src/components/ReportBugWidget.tsx`

- [ ] **Step 1: Read `website/src/components/ReportBugWidget.tsx`** to copy its conventions: the `API_URL` resolution (`import.meta.env.PUBLIC_API_URL || 'https://api.draftright.info'`), `getAccessToken()` reading `localStorage['dr_access_token']`, the button → modal/panel pattern, and styling classes.

- [ ] **Step 2: Create `website/src/components/SuggestFeatureWidget.tsx`**

```tsx
import { useState } from 'react';

const API_URL =
  (typeof import.meta !== 'undefined' && (import.meta as any).env?.PUBLIC_API_URL) ||
  'https://api.draftright.info';
const BOARD_URL = 'https://draftright.info/feedback';

const PLATFORMS: Array<{ value: string; label: string }> = [
  { value: 'playground', label: 'Playground (web)' },
  { value: 'mobile', label: 'Mobile (iOS / Android)' },
  { value: 'windows', label: 'Windows' },
  { value: 'mac', label: 'macOS' },
  { value: 'linux', label: 'Linux' },
];

function getAccessToken(): string | null {
  try { return localStorage.getItem('dr_access_token'); } catch { return null; }
}

/**
 * Compact "Suggest a feature" launcher + modal for the web playground.
 * Mirrors ReportBugWidget but posts JSON to POST /feedback with
 * kind:"feature" and a target_platform the user picks from a dropdown.
 */
export default function SuggestFeatureWidget() {
  const [open, setOpen] = useState(false);
  const [title, setTitle] = useState('');
  const [platform, setPlatform] = useState('playground');
  const [description, setDescription] = useState('');
  const [email, setEmail] = useState('');
  const [busy, setBusy] = useState(false);
  const [done, setDone] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const token = getAccessToken();
  const canSubmit = title.trim().length > 0 && description.trim().length > 0 && !busy;

  async function submit(e: React.FormEvent) {
    e.preventDefault();
    if (!canSubmit) return;
    setBusy(true); setError(null);
    try {
      const headers: Record<string, string> = { 'Content-Type': 'application/json' };
      if (token) headers['Authorization'] = `Bearer ${token}`;
      const body: Record<string, unknown> = {
        kind: 'feature',
        title: title.trim(),
        target_platform: platform,
        description: description.trim(),
        source: 'web',
      };
      if (!token && email.trim()) body.user_email = email.trim();
      const res = await fetch(`${API_URL}/feedback`, {
        method: 'POST', headers, body: JSON.stringify(body),
      });
      if (!res.ok) throw new Error(`server returned ${res.status}`);
      setDone(true);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'something went wrong');
    } finally {
      setBusy(false);
    }
  }

  function reset() {
    setOpen(false); setDone(false); setError(null);
    setTitle(''); setDescription(''); setEmail(''); setPlatform('playground');
  }

  return (
    <div className="suggest-feature-widget" style={{ marginTop: '0.75rem' }}>
      <button type="button" onClick={() => setOpen(true)}
        style={{ fontSize: '0.85rem', background: 'transparent', border: '1px solid #334155',
                 color: '#94a3b8', borderRadius: 8, padding: '6px 12px', cursor: 'pointer' }}>
        💡 Suggest a feature
      </button>
      {open && (
        <div role="dialog" aria-modal="true"
          style={{ position: 'fixed', inset: 0, background: '#0008', display: 'flex',
                   alignItems: 'center', justifyContent: 'center', padding: 20, zIndex: 1000 }}
          onClick={(e) => { if (e.target === e.currentTarget) reset(); }}>
          <div style={{ background: '#161b22', border: '1px solid #2a3240', borderRadius: 14,
                        width: '100%', maxWidth: 460, padding: 20, color: '#e6edf3' }}>
            {done ? (
              <>
                <h3 style={{ marginBottom: 8 }}>Thanks!</h3>
                <p style={{ color: '#94a3b8', fontSize: 14, marginBottom: 16 }}>
                  Your feature request was submitted. Track it on the board.
                </p>
                <div style={{ display: 'flex', gap: 8, justifyContent: 'flex-end' }}>
                  <a href={BOARD_URL} target="_blank" rel="noopener"
                     style={{ color: '#6aa6ff', alignSelf: 'center', fontSize: 13 }}>See all requests →</a>
                  <button onClick={reset} style={{ background: '#3b82f6', color: '#fff',
                    border: 0, borderRadius: 9, padding: '8px 14px', cursor: 'pointer' }}>Close</button>
                </div>
              </>
            ) : (
              <form onSubmit={submit}>
                <h3 style={{ marginBottom: 4 }}>Suggest a feature</h3>
                <p style={{ color: '#94a3b8', fontSize: 12, marginBottom: 16 }}>
                  {token ? 'Submitted under your account. ' : ''}Public on the feature board.
                </p>
                <label style={{ fontSize: 12, color: '#94a3b8' }}>Title</label>
                <input value={title} maxLength={80} onChange={(e) => setTitle(e.target.value)}
                  placeholder="One line — what should we build?"
                  style={{ width: '100%', margin: '5px 0 14px', background: '#1c2230',
                           border: '1px solid #2a3240', borderRadius: 9, color: '#e6edf3', padding: '9px 11px' }} />
                <label style={{ fontSize: 12, color: '#94a3b8' }}>Which platform is this for?</label>
                <select value={platform} onChange={(e) => setPlatform(e.target.value)}
                  style={{ width: '100%', margin: '5px 0 14px', background: '#1c2230',
                           border: '1px solid #2a3240', borderRadius: 9, color: '#e6edf3', padding: '9px 11px' }}>
                  {PLATFORMS.map((p) => <option key={p.value} value={p.value}>{p.label}</option>)}
                </select>
                <label style={{ fontSize: 12, color: '#94a3b8' }}>Details</label>
                <textarea value={description} maxLength={2000} onChange={(e) => setDescription(e.target.value)}
                  placeholder="What problem does it solve? How would it work?"
                  style={{ width: '100%', margin: '5px 0 14px', minHeight: 90, background: '#1c2230',
                           border: '1px solid #2a3240', borderRadius: 9, color: '#e6edf3', padding: '9px 11px', resize: 'vertical' }} />
                {!token && (
                  <>
                    <label style={{ fontSize: 12, color: '#94a3b8' }}>Email (optional — to follow up)</label>
                    <input value={email} type="email" onChange={(e) => setEmail(e.target.value)}
                      style={{ width: '100%', margin: '5px 0 14px', background: '#1c2230',
                               border: '1px solid #2a3240', borderRadius: 9, color: '#e6edf3', padding: '9px 11px' }} />
                  </>
                )}
                {error && <p style={{ color: '#f87171', fontSize: 13, marginBottom: 10 }}>{error}</p>}
                <div style={{ display: 'flex', gap: 8, justifyContent: 'flex-end', alignItems: 'center' }}>
                  <a href={BOARD_URL} target="_blank" rel="noopener" style={{ color: '#6aa6ff', fontSize: 13, marginRight: 'auto' }}>See all requests →</a>
                  <button type="button" onClick={reset} style={{ background: 'transparent',
                    border: '1px solid #2a3240', color: '#94a3b8', borderRadius: 9, padding: '8px 14px', cursor: 'pointer' }}>Cancel</button>
                  <button type="submit" disabled={!canSubmit} style={{ background: canSubmit ? '#22c55e' : '#1c2230',
                    color: canSubmit ? '#04210f' : '#475569', border: 0, borderRadius: 9, padding: '8px 16px',
                    fontWeight: 700, cursor: canSubmit ? 'pointer' : 'not-allowed' }}>
                    {busy ? 'Submitting…' : 'Submit request'}
                  </button>
                </div>
              </form>
            )}
          </div>
        </div>
      )}
    </div>
  );
}
```

- [ ] **Step 3: Render it in `website/src/components/Playground.tsx`** — find the line that renders `<ReportBugWidget … />` (around line 328) and add directly below it:

```tsx
        <SuggestFeatureWidget />
```

and add the import at the top with the other component imports:

```tsx
import SuggestFeatureWidget from './SuggestFeatureWidget';
```

- [ ] **Step 4: Build the site**

Run: `cd website && npm run build`
Expected: build succeeds, no TypeScript/Astro errors.

- [ ] **Step 5: Manual smoke (optional but recommended)**

Run: `cd website && npm run dev`, open the playground, click "💡 Suggest a feature", fill title + pick a platform + details, submit. With the backend running locally and `PUBLIC_API_URL=http://localhost:3000`, expect the "Thanks!" panel and a new `kind=feature` row via `curl http://localhost:3000/feedback`.

- [ ] **Step 6: Commit**

```bash
git add website/src/components/SuggestFeatureWidget.tsx website/src/components/Playground.tsx
git commit -m "feat(website): 'Suggest a feature' widget on the playground → POST /feedback"
```

---

### Task 2: macOS app — `SuggestFeatureSheet` + service method + menu/Settings entries

**Files:**
- Create: `DraftRight/UI/SuggestFeatureSheet.swift`
- Modify: `DraftRight/Services/BugReportService.swift` (add a `submitFeatureRequest` static func — or create `DraftRight/Services/FeedbackService.swift`; this plan adds it to a new `FeedbackService.swift` for clarity)
- Create: `DraftRight/Services/FeedbackService.swift`
- Modify: `DraftRight/DraftRightApp.swift` (add a "Suggest a Feature…" menu item near "Report a Bug…")
- Modify: `DraftRight/UI/Settings/AdvancedSettingsTab.swift` (add a "Suggest a Feature…" button near the bug-report button)
- Reference (read, don't change): `DraftRight/UI/BugReportSheet.swift`, `DraftRight/Services/BugReportService.swift`, `DraftRight/AppModel.swift` (for `backendUrl`, `accessToken`)

- [ ] **Step 1: Read** `DraftRight/UI/BugReportSheet.swift` (the `BugReportSheet` view + `BugReportPresenter`), `DraftRight/Services/BugReportService.swift`, and how `backendUrl`/`accessToken` are exposed on `AppModel`.

- [ ] **Step 2: Create `DraftRight/Services/FeedbackService.swift`**

```swift
import Foundation

/// Posts feature requests to the backend `POST /feedback` endpoint
/// (JSON; no screenshot). Mirrors `BugReportService` but for feature
/// requests rather than bug reports.
enum FeedbackService {
    enum FeedbackError: Error { case badResponse(Int), invalidURL }

    struct CreateResponse: Decodable { let id: String }

    /// Submit a feature request. `targetPlatform` must be one of
    /// playground|mobile|windows|mac|linux. `authToken` is sent as a
    /// Bearer token when non-nil.
    static func submitFeatureRequest(
        title: String,
        targetPlatform: String,
        description: String,
        userEmail: String?,
        authToken: String?,
        backendUrl: String
    ) async throws -> String {
        guard let url = URL(string: backendUrl.trimmingCharacters(in: .whitespaces)
                            .appending("/feedback")) else { throw FeedbackError.invalidURL }
        var req = URLRequest(url: url)
        req.httpMethod = "POST"
        req.setValue("application/json", forHTTPHeaderField: "Content-Type")
        if let token = authToken, !token.isEmpty {
            req.setValue("Bearer \(token)", forHTTPHeaderField: "Authorization")
        }
        var body: [String: Any] = [
            "kind": "feature",
            "title": title.trimmingCharacters(in: .whitespacesAndNewlines),
            "target_platform": targetPlatform,
            "description": description.trimmingCharacters(in: .whitespacesAndNewlines),
            "source": "macos-app",
        ]
        if (authToken ?? "").isEmpty, let email = userEmail?.trimmingCharacters(in: .whitespaces),
           !email.isEmpty { body["user_email"] = email }
        req.httpBody = try JSONSerialization.data(withJSONObject: body)

        let (data, resp) = try await URLSession.shared.data(for: req)
        guard let http = resp as? HTTPURLResponse, (200...299).contains(http.statusCode) else {
            throw FeedbackError.badResponse((resp as? HTTPURLResponse)?.statusCode ?? -1)
        }
        let decoded = try JSONDecoder().decode(CreateResponse.self, from: data)
        return decoded.id
    }
}
```

- [ ] **Step 3: Create `DraftRight/UI/SuggestFeatureSheet.swift`**

Model it on `BugReportSheet`/`BugReportPresenter` but with: a `title` `TextField`, a `Picker` for the target platform (default `.mac`), a `description` `TextEditor`, an optional email field shown only when not logged in, a "See all requests →" link button that calls `NSWorkspace.shared.open(URL(string: "https://draftright.info/feedback")!)`, a submit button disabled until title + description are non-empty, and a `FeedbackPresenter` that opens a floating `NSWindow` exactly like `BugReportPresenter` (the app is `LSUIElement`, so an attached sheet isn't available).

```swift
import SwiftUI
import AppKit

private let feedbackPlatforms: [(value: String, label: String)] = [
    ("playground", "Playground (web)"),
    ("mobile", "Mobile (iOS / Android)"),
    ("windows", "Windows"),
    ("mac", "macOS"),
    ("linux", "Linux"),
]
private let feedbackBoardURL = URL(string: "https://draftright.info/feedback")!

/// "Suggest a feature" form — title + target-platform picker + details.
/// Opened from the menu bar and the Advanced settings tab.
struct SuggestFeatureSheet: View {
    let appModel: AppModel
    var onClose: () -> Void

    @State private var title = ""
    @State private var platform = "mac"
    @State private var details = ""
    @State private var email = ""
    @State private var busy = false
    @State private var done = false
    @State private var errorText: String?

    private var isLoggedIn: Bool { (appModel.accessToken ?? "").isEmpty == false }
    private var canSubmit: Bool {
        !busy && !title.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty
        && !details.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty
    }

    var body: some View {
        VStack(alignment: .leading, spacing: 12) {
            Text("Suggest a feature").font(.headline)
            if done {
                Text("Thanks! Your feature request was submitted.").foregroundColor(.secondary)
            } else {
                TextField("One line — what should we build?", text: $title)
                Picker("For platform", selection: $platform) {
                    ForEach(feedbackPlatforms, id: \.value) { Text($0.label).tag($0.value) }
                }
                Text("Details").font(.caption).foregroundColor(.secondary)
                TextEditor(text: $details).frame(minHeight: 90).border(Color.secondary.opacity(0.3))
                if !isLoggedIn {
                    TextField("Email (optional — to follow up)", text: $email)
                }
                if let e = errorText { Text(e).foregroundColor(.red).font(.caption) }
            }
            HStack {
                Button("See all requests →") { NSWorkspace.shared.open(feedbackBoardURL) }
                    .buttonStyle(.link)
                Spacer()
                Button(done ? "Close" : "Cancel") { onClose() }
                if !done {
                    Button(busy ? "Submitting…" : "Submit request") { Task { await submit() } }
                        .disabled(!canSubmit).keyboardShortcut(.defaultAction)
                }
            }
        }
        .padding(20).frame(width: 420)
    }

    private func submit() async {
        busy = true; errorText = nil
        do {
            _ = try await FeedbackService.submitFeatureRequest(
                title: title, targetPlatform: platform, description: details,
                userEmail: isLoggedIn ? nil : email,
                authToken: appModel.accessToken, backendUrl: appModel.backendUrl)
            done = true
        } catch {
            errorText = "Couldn't submit — \(error)"
        }
        busy = false
    }
}

/// Opens `SuggestFeatureSheet` in a floating window (same approach as
/// `BugReportPresenter` — the app is LSUIElement so it has no normal
/// window to attach a sheet to).
enum FeedbackPresenter {
    private static var window: NSWindow?
    static func present(appModel: AppModel) {
        if let w = window { w.makeKeyAndOrderFront(nil); NSApp.activate(ignoringOtherApps: true); return }
        let w = NSWindow(contentRect: NSRect(x: 0, y: 0, width: 420, height: 360),
                         styleMask: [.titled, .closable], backing: .buffered, defer: false)
        w.title = "Suggest a Feature"
        w.isReleasedWhenClosed = false
        w.center()
        w.level = .floating
        let view = SuggestFeatureSheet(appModel: appModel) { window?.close(); window = nil }
        w.contentView = NSHostingView(rootView: view)
        window = w
        w.makeKeyAndOrderFront(nil)
        NSApp.activate(ignoringOtherApps: true)
    }
}
```

(If `BugReportPresenter` in this codebase does something different — e.g. wraps the view differently or sets specific window flags — match that pattern instead. Read it first.)

- [ ] **Step 4: Add the menu item** in `DraftRight/DraftRightApp.swift` — find the `Button("Report a Bug…") { BugReportPresenter.present(appModel: appModel) }` line (~line 45 in the `MenuBarExtra` body) and add directly below it:

```swift
            Button("Suggest a Feature…") { FeedbackPresenter.present(appModel: appModel) }
```

- [ ] **Step 5: Add the Settings button** in `DraftRight/UI/Settings/AdvancedSettingsTab.swift` — find the existing bug-report button (`BugReportPresenter.present(appModel: appModel)` ~line 28) and add a sibling:

```swift
                Button("Suggest a Feature…") { FeedbackPresenter.present(appModel: appModel) }
```

(place it right after the bug-report button, inside the same section/`HStack`/`VStack`).

- [ ] **Step 6: Build**

Run: `cd DraftRight && swift build` (or open in Xcode and build the macOS target).
Expected: builds clean, no errors.

- [ ] **Step 7: Manual smoke (optional)**

Run the app, open the menu bar → "Suggest a Feature…", submit a request against a local backend (`backendUrl` set to `http://localhost:3000` in Settings). Expect "Thanks!" and a new row via `curl http://localhost:3000/feedback`.

- [ ] **Step 8: Commit**

```bash
git add DraftRight/Services/FeedbackService.swift DraftRight/UI/SuggestFeatureSheet.swift DraftRight/DraftRightApp.swift DraftRight/UI/Settings/AdvancedSettingsTab.swift
git commit -m "feat(macos): 'Suggest a feature' sheet (menu bar + Settings) → POST /feedback"
```

---

### Task 3: Windows app — `SuggestFeatureDialog` + service method + Settings entry

**Files:**
- Create: `DraftRightWindows/DraftRightWindows/Views/SuggestFeatureDialog.cs`
- Create: `DraftRightWindows/DraftRightWindows/Services/FeedbackService.cs`
- Modify: `DraftRightWindows/DraftRightWindows/App.cs` (in `BuildAdvancedTab()`, in the existing "Feedback" section, add a "Suggest a feature…" button next to the bug-report button)
- Reference (read, don't change): `DraftRightWindows/DraftRightWindows/Views/ReportBugDialog.cs`, `DraftRightWindows/DraftRightWindows/Services/BugReportService.cs`, `DraftRightWindows/DraftRightWindows/Constants.cs` (`DefaultBackendUrl`), how `App.Settings.BackendUrl` and the auth token are accessed in `BugReportService.SubmitAsync`.

- [ ] **Step 1: Read** `ReportBugDialog.cs` (the WinForms `Form` it builds on an STA thread), `BugReportService.cs` (how it resolves `baseUrl`, gets the auth token, builds the request), and `Constants.cs`.

- [ ] **Step 2: Create `DraftRightWindows/DraftRightWindows/Services/FeedbackService.cs`**

```csharp
using System;
using System.Net.Http;
using System.Net.Http.Headers;
using System.Text;
using System.Text.Json;
using System.Threading.Tasks;

namespace DraftRightWindows.Services
{
    /// <summary>
    /// Posts feature requests to the backend <c>POST /feedback</c> endpoint
    /// (JSON; no screenshot). Mirrors <see cref="BugReportService"/> for
    /// feature requests instead of bug reports.
    /// </summary>
    internal static class FeedbackService
    {
        public readonly record struct SubmitResult(bool Ok, string? Id, string? Error);

        private static readonly HttpClient Http = new();

        /// <param name="targetPlatform">playground|mobile|windows|mac|linux</param>
        public static async Task<SubmitResult> SubmitAsync(
            string title, string targetPlatform, string description,
            string? userEmail, string? authToken, string? backendUrlOverride = null)
        {
            try
            {
                var baseUrl = (backendUrlOverride ?? App.Settings?.BackendUrl ?? Constants.DefaultBackendUrl)
                    .TrimEnd('/');
                var payload = new System.Collections.Generic.Dictionary<string, object?>
                {
                    ["kind"] = "feature",
                    ["title"] = (title ?? "").Trim(),
                    ["target_platform"] = targetPlatform,
                    ["description"] = (description ?? "").Trim(),
                    ["source"] = "windows-app",
                };
                if (string.IsNullOrWhiteSpace(authToken) && !string.IsNullOrWhiteSpace(userEmail))
                    payload["user_email"] = userEmail!.Trim();

                using var req = new HttpRequestMessage(HttpMethod.Post, $"{baseUrl}/feedback")
                {
                    Content = new StringContent(JsonSerializer.Serialize(payload), Encoding.UTF8, "application/json"),
                };
                if (!string.IsNullOrWhiteSpace(authToken))
                    req.Headers.Authorization = new AuthenticationHeaderValue("Bearer", authToken);

                using var resp = await Http.SendAsync(req).ConfigureAwait(false);
                var bodyText = await resp.Content.ReadAsStringAsync().ConfigureAwait(false);
                if (!resp.IsSuccessStatusCode)
                    return new SubmitResult(false, null, $"server returned {(int)resp.StatusCode}");
                string? id = null;
                try { id = JsonDocument.Parse(bodyText).RootElement.GetProperty("id").GetString(); } catch { }
                return new SubmitResult(true, id, null);
            }
            catch (Exception ex)
            {
                return new SubmitResult(false, null, ex.Message);
            }
        }
    }
}
```

(Adjust `App.Settings?.BackendUrl` / `Constants.DefaultBackendUrl` to match exactly how `BugReportService.cs` resolves the base URL and how it reads the auth token — read that file and mirror it.)

- [ ] **Step 3: Create `DraftRightWindows/DraftRightWindows/Views/SuggestFeatureDialog.cs`**

Mirror `ReportBugDialog.cs`: an `internal static class SuggestFeatureDialog` with `static void Show(IntPtr ownerHwnd = default)` that builds a dark-themed WinForms `Form` on a new STA thread. Fields, top to bottom: a `TextBox` for **Title** (`MaxLength = 80`), a `ComboBox` (`DropDownStyle = ComboBoxStyle.DropDownList`) for **target platform** with items Playground/Mobile/Windows/macOS/Linux mapped to `playground|mobile|windows|mac|linux`, default-selected to `windows`; a multiline `TextBox` for **Details** (`MaxLength = 2000`); an email `TextBox` shown only when not signed in; a "See all requests →" `LinkLabel` whose click does `System.Diagnostics.Process.Start(new ProcessStartInfo("https://draftright.info/feedback") { UseShellExecute = true })`; a Submit button (disabled until title + details non-empty) that calls `await FeedbackService.SubmitAsync(...)` on the UI thread (use `async void` event handler like `ReportBugDialog` does), shows a "Thanks!" state or an error label, and a Cancel/Close button. Determine "signed in" the same way `ReportBugDialog` does (it hides its email field when signed in).

Keep the file structurally parallel to `ReportBugDialog.cs` — same threading, same theming helpers, same control layout idiom. Don't invent a new UI framework path.

- [ ] **Step 4: Wire the Settings button** in `DraftRightWindows/DraftRightWindows/App.cs` — in `BuildAdvancedTab()`, in the "Feedback" section, near `bugBtn.Click += (_, _) => Views.ReportBugDialog.Show();` (~line 1454), add:

```csharp
            var featureBtn = new WinForms.Button { Text = "Suggest a feature…", AutoSize = true };
            // (style featureBtn to match bugBtn — copy whatever theming bugBtn gets)
            featureBtn.Click += (_, _) => Views.SuggestFeatureDialog.Show();
```

and add `featureBtn` to the same container `bugBtn` is added to, right after it. Match the exact button construction/styling used for `bugBtn` in that method (read it).

- [ ] **Step 5: Build** — push to the `develop` branch and let GitHub Actions "Build Windows app" compile it (there is no Windows SDK on the Mac dev box; the workflow runs on push). OR if a Windows toolchain is available: `dotnet build DraftRightWindows/DraftRightWindows/DraftRightWindows.csproj`. Expected: build succeeds. (If relying on CI, this step is verified by the CI run going green — note the run id.)

- [ ] **Step 6: Commit**

```bash
git add DraftRightWindows/DraftRightWindows/Services/FeedbackService.cs DraftRightWindows/DraftRightWindows/Views/SuggestFeatureDialog.cs DraftRightWindows/DraftRightWindows/App.cs
git commit -m "feat(windows): 'Suggest a feature' dialog in Settings → POST /feedback"
```

---

### Task 4: Flutter mobile — `feedback_service.dart` + `showSuggestFeatureSheet` + Settings entry

**Files:**
- Create: `DraftRightMobile/lib/services/feedback_service.dart`
- Create: `DraftRightMobile/lib/widgets/suggest_feature_sheet.dart`
- Modify: `DraftRightMobile/lib/screens/settings_screen.dart` (add a "Suggest a feature" `ListTile` near the "Report a bug" one, ~line 237)
- Test: `DraftRightMobile/test/feedback_service_test.dart` (create — a unit test for the payload/URL builder)
- Reference (read, don't change): `DraftRightMobile/lib/services/bug_report_service.dart`, `DraftRightMobile/lib/widgets/report_bug_sheet.dart`, `DraftRightMobile/lib/services/auth_service.dart` (for `accessToken`).

- [ ] **Step 1: Read** `bug_report_service.dart` (its `submitBugReport` signature, the `source` auto-detection `ios-app`/`android-app`, `endpointOverride`), `report_bug_sheet.dart` (`showReportBugSheet`, the Cupertino-vs-Material modal split, how it reads `context.read<AuthService>().accessToken`), and how `settings_screen.dart` invokes `showReportBugSheet`.

- [ ] **Step 2: Write the failing test** — `DraftRightMobile/test/feedback_service_test.dart`:

```dart
import 'dart:convert';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:draftright/services/feedback_service.dart';

void main() {
  test('submitFeatureRequest posts JSON kind=feature with the picked platform', () async {
    http.Request? captured;
    final client = MockClient((req) async {
      captured = req as http.Request;
      return http.Response(jsonEncode({'id': 'feat-1', 'message': 'ok'}), 201,
          headers: {'content-type': 'application/json'});
    });

    final ok = await FeedbackService.submitFeatureRequest(
      title: 'Dark mode',
      targetPlatform: 'linux',
      description: 'please follow system theme',
      authToken: 'tok-123',
      endpointOverride: 'http://localhost:9/feedback',
      httpClient: client,
    );

    expect(ok, isTrue);
    expect(captured!.url.toString(), 'http://localhost:9/feedback');
    expect(captured!.headers['authorization'], 'Bearer tok-123');
    final body = jsonDecode(captured!.body) as Map<String, dynamic>;
    expect(body['kind'], 'feature');
    expect(body['title'], 'Dark mode');
    expect(body['target_platform'], 'linux');
    expect(body['description'], 'please follow system theme');
    expect(body['source'], anyOf('ios-app', 'android-app'));
  });

  test('submitFeatureRequest returns false on a non-2xx response', () async {
    final client = MockClient((req) async => http.Response('bad', 400));
    final ok = await FeedbackService.submitFeatureRequest(
      title: 'X', targetPlatform: 'mac', description: 'd',
      endpointOverride: 'http://localhost:9/feedback', httpClient: client,
    );
    expect(ok, isFalse);
  });

  test('submitFeatureRequest includes user_email only when no auth token', () async {
    Map<String, dynamic>? body;
    final client = MockClient((req) async {
      body = jsonDecode((req as http.Request).body) as Map<String, dynamic>;
      return http.Response(jsonEncode({'id': 'x'}), 201);
    });
    await FeedbackService.submitFeatureRequest(
      title: 'X', targetPlatform: 'mac', description: 'd',
      userEmail: 'a@b.c', endpointOverride: 'http://localhost:9/feedback', httpClient: client);
    expect(body!.containsKey('user_email'), isTrue);

    body = null;
    await FeedbackService.submitFeatureRequest(
      title: 'X', targetPlatform: 'mac', description: 'd', userEmail: 'a@b.c',
      authToken: 'tok', endpointOverride: 'http://localhost:9/feedback', httpClient: client);
    expect(body!.containsKey('user_email'), isFalse);
  });
}

/// Minimal http.BaseClient that delegates to a handler — avoids a real socket.
class MockClient extends http.BaseClient {
  final Future<http.Response> Function(http.BaseRequest) handler;
  MockClient(this.handler);
  @override
  Future<http.StreamedResponse> send(http.BaseRequest request) async {
    if (request is http.Request) {
      // ignore: invalid_use_of_protected_member
    }
    final r = await handler(request);
    return http.StreamedResponse(Stream.value(utf8.encode(r.body)), r.statusCode,
        headers: r.headers, request: request);
  }
}
```

(If the project already has a mock-http helper, use that instead of this `MockClient`. Check `DraftRightMobile/test/` for existing patterns — e.g. `bug_report_test.dart` spins a real stub server; a `package:http` `BaseClient` mock is simpler for a unit test.)

- [ ] **Step 3: Run the test — fails**

Run: `cd DraftRightMobile && flutter test test/feedback_service_test.dart`
Expected: FAIL — `feedback_service.dart` doesn't exist.

- [ ] **Step 4: Create `DraftRightMobile/lib/services/feedback_service.dart`**

```dart
import 'dart:convert';
import 'dart:io' show Platform;
import 'package:http/http.dart' as http;

/// Posts feature requests to the backend `POST /feedback` endpoint
/// (JSON; no screenshot). The bug-report counterpart is
/// `BugReportService.submitBugReport`.
class FeedbackService {
  static const String _defaultEndpoint = 'https://api.draftright.info/feedback';

  /// Submit a feature request. [targetPlatform] is one of
  /// playground|mobile|windows|mac|linux (picked by the user in the
  /// dropdown). [authToken] is sent as a Bearer token when non-null.
  /// [endpointOverride] points the POST at an alternate URL (tests).
  /// [httpClient] is injectable for tests; defaults to a fresh client.
  static Future<bool> submitFeatureRequest({
    required String title,
    required String targetPlatform,
    required String description,
    String? userEmail,
    String? authToken,
    String? endpointOverride,
    http.Client? httpClient,
  }) async {
    final client = httpClient ?? http.Client();
    try {
      final source = Platform.isIOS ? 'ios-app' : 'android-app';
      final body = <String, dynamic>{
        'kind': 'feature',
        'title': title.trim(),
        'target_platform': targetPlatform,
        'description': description.trim(),
        'source': source,
      };
      if ((authToken == null || authToken.isEmpty) &&
          userEmail != null && userEmail.trim().isNotEmpty) {
        body['user_email'] = userEmail.trim();
      }
      final headers = <String, String>{'Content-Type': 'application/json'};
      if (authToken != null && authToken.isNotEmpty) {
        headers['Authorization'] = 'Bearer $authToken';
      }
      final resp = await client.post(
        Uri.parse(endpointOverride ?? _defaultEndpoint),
        headers: headers, body: jsonEncode(body),
      );
      return resp.statusCode >= 200 && resp.statusCode < 300;
    } catch (_) {
      return false;
    } finally {
      if (httpClient == null) client.close();
    }
  }
}
```

Note: `Platform.isIOS` throws on web/tests-without-a-platform — but Flutter unit tests run on the host VM where `Platform` is available (it'll report the host OS, so the `anyOf('ios-app','android-app')` matcher in the test handles it). If a test environment complains, the test can pass `endpointOverride` and just not assert `source` strictly — but the matcher above already tolerates it.

- [ ] **Step 5: Run the test — passes**

Run: `cd DraftRightMobile && flutter test test/feedback_service_test.dart`
Expected: PASS (3 tests).

- [ ] **Step 6: Create `DraftRightMobile/lib/widgets/suggest_feature_sheet.dart`**

Mirror `report_bug_sheet.dart`: a top-level `Future<void> showSuggestFeatureSheet(BuildContext context, {String? endpointOverride})` that on iOS uses `showCupertinoModalPopup` and on Android `showModalBottomSheet`, presenting a private `_SuggestFeatureSheet` stateful widget with: a **Title** `TextField` (`maxLength: 80`), a platform `DropdownButton<String>` with items `playground|mobile|windows|mac|linux` (labels Playground/Mobile/Windows/macOS/Linux), default `'mobile'`; a **Details** `TextField` (`maxLength: 2000`, multiline); an email field shown only when `context.read<AuthService>().accessToken == null`; a "See all requests →" `TextButton` that does `launchUrl(Uri.parse('https://draftright.info/feedback'), mode: LaunchMode.externalApplication)` (the project already uses `url_launcher` — check `pubspec.yaml`); a Submit button disabled until title + details non-empty that calls `FeedbackService.submitFeatureRequest(... authToken: auth.accessToken, endpointOverride: endpointOverride)`, then on success shows a `SnackBar('Feature request submitted — thanks!')` and pops the sheet, on failure shows `SnackBar('Couldn't submit — try again')` and keeps it open. Wrap with the same `AuthService` provider read pattern `report_bug_sheet.dart` uses.

- [ ] **Step 7: Wire the Settings entry** in `DraftRightMobile/lib/screens/settings_screen.dart` — near the `ListTile` that calls `showReportBugSheet(context, …)` (~line 237), add another `ListTile`:

```dart
            ListTile(
              leading: const Icon(Icons.lightbulb_outline),
              title: const Text('Suggest a feature'),
              onTap: () => showSuggestFeatureSheet(context),
            ),
```

and add `import '../widgets/suggest_feature_sheet.dart';` at the top.

- [ ] **Step 8: Analyze + full test run**

Run: `cd DraftRightMobile && flutter analyze && flutter test`
Expected: no analyzer errors; all tests pass (including the new `feedback_service_test.dart` and the existing `bug_report_test.dart`).

- [ ] **Step 9: Commit**

```bash
git add DraftRightMobile/lib/services/feedback_service.dart DraftRightMobile/lib/widgets/suggest_feature_sheet.dart DraftRightMobile/lib/screens/settings_screen.dart DraftRightMobile/test/feedback_service_test.dart
git commit -m "feat(mobile): 'Suggest a feature' sheet + FeedbackService → POST /feedback"
```

---

### Task 5: Linux app — new feedback dialog + service + Settings/tray entry

**Files:**
- Create: `DraftRightLinux/draftright/services/feedback_service.py`
- Create: `DraftRightLinux/draftright/ui/suggest_feature_dialog.py`
- Modify: `DraftRightLinux/draftright/ui/settings_window.py` (add a "Suggest a feature…" row — an `Adw.ActionRow` with a button — to a preferences page)
- Modify: `DraftRightLinux/draftright/ui/tray_icon.py` (optional: add a "Suggest a feature…" menu item near "Settings")
- Reference (read, don't change): `DraftRightLinux/draftright/services/settings_service.py` (`backend_url`), `DraftRightLinux/draftright/services/error_reporter.py` (how it builds an HTTP request — and where the auth token lives — there should be an auth/session service; find it, e.g. `services/auth_service.py` or similar), `DraftRightLinux/draftright/ui/settings_window.py` (the libadwaita preferences-page idiom).

- [ ] **Step 1: Read** `settings_service.py`, `error_reporter.py`, the auth/session service (locate it), and `settings_window.py`. Note: Linux has **no** existing bug-report UI, so there's no template — follow the libadwaita patterns already in `settings_window.py` and the HTTP pattern in `error_reporter.py`.

- [ ] **Step 2: Create `DraftRightLinux/draftright/services/feedback_service.py`**

```python
"""Submits feature requests to the backend POST /feedback endpoint (JSON)."""
from __future__ import annotations

import json
import urllib.request
import urllib.error

from .settings_service import settings_service

# Resolve the auth/session service the way the rest of the app does.
# If the project exposes the access token elsewhere, import that instead.
try:
    from .auth_service import auth_service  # type: ignore
except Exception:  # pragma: no cover - fallback if module name differs
    auth_service = None  # type: ignore

_TARGET_PLATFORMS = ("playground", "mobile", "windows", "mac", "linux")


def submit_feature_request(
    *, title: str, target_platform: str, description: str,
    user_email: str | None = None, timeout: float = 15.0,
) -> str:
    """POST a feature request. Returns the new row id.

    Raises RuntimeError on a non-2xx response or transport error.
    `target_platform` must be one of playground|mobile|windows|mac|linux.
    """
    if target_platform not in _TARGET_PLATFORMS:
        raise ValueError(f"target_platform must be one of {_TARGET_PLATFORMS}")
    base = settings_service.backend_url.rstrip("/")
    token = getattr(auth_service, "access_token", None) if auth_service else None

    payload: dict[str, object] = {
        "kind": "feature",
        "title": title.strip(),
        "target_platform": target_platform,
        "description": description.strip(),
        "source": "linux-app",
    }
    if not token and user_email and user_email.strip():
        payload["user_email"] = user_email.strip()

    data = json.dumps(payload).encode("utf-8")
    req = urllib.request.Request(f"{base}/feedback", data=data, method="POST")
    req.add_header("Content-Type", "application/json")
    if token:
        req.add_header("Authorization", f"Bearer {token}")
    try:
        with urllib.request.urlopen(req, timeout=timeout) as resp:
            body = json.loads(resp.read().decode("utf-8"))
            return str(body.get("id", ""))
    except urllib.error.HTTPError as e:
        raise RuntimeError(f"server returned {e.code}") from e
    except urllib.error.URLError as e:
        raise RuntimeError(f"network error: {e.reason}") from e
```

(Adjust the auth-token access — `getattr(auth_service, "access_token", None)` — to match exactly how the Linux app stores the logged-in user's token. If there is no session/token concept on Linux yet, drop the token logic and always send `user_email` when provided.)

- [ ] **Step 3: Create `DraftRightLinux/draftright/ui/suggest_feature_dialog.py`**

A GTK4/libadwaita modal — `class SuggestFeatureDialog(Adw.Window)` (or `Adw.MessageDialog` if that's the codebase's idiom; check `settings_window.py`'s siblings) — containing: an `Adw.EntryRow` (or `Gtk.Entry`) for **Title**; a `Gtk.DropDown` (from a `Gtk.StringList` of "Playground", "Mobile", "Windows", "macOS", "Linux", mapped to `playground|mobile|windows|mac|linux`), default-selected to index 4 (`linux`); a `Gtk.TextView` (wrapped in a `Gtk.ScrolledWindow`) for **Details**; an email `Gtk.Entry` shown only when not signed in (skip entirely if Linux has no session concept — then always show it as optional); a "See all requests →" `Gtk.LinkButton` with URI `https://draftright.info/feedback`; "Cancel" and "Submit" buttons. On Submit: disable the button, call `feedback_service.submit_feature_request(...)` on a worker thread (`GLib.Thread` / `threading.Thread` + `GLib.idle_add` to marshal the result back — match whatever async pattern `error_reporter.py` or the rewrite call uses), then on success show a toast/`Adw.Toast` "Feature request submitted — thanks!" and close; on error show the error in a label and keep the dialog open. Constructor takes the parent window for `set_transient_for`.

Provide a module-level helper `def open_suggest_feature_dialog(parent) -> None:` that constructs and `present()`s the dialog.

- [ ] **Step 4: Add the Settings entry** in `DraftRightLinux/draftright/ui/settings_window.py` — add to an appropriate `Adw.PreferencesGroup` (e.g. an "About"/"Help" group, creating one if none exists) an `Adw.ActionRow` titled "Suggest a feature" with a `Gtk.Button` (label "Open…", `valign=Gtk.Align.CENTER`) as its suffix widget; on `clicked`, call `from .suggest_feature_dialog import open_suggest_feature_dialog; open_suggest_feature_dialog(self)`.

- [ ] **Step 5: (Optional) Add the tray entry** in `DraftRightLinux/draftright/ui/tray_icon.py` — in `_build_menu()`, add a menu item "Suggest a feature…" right after "Settings" that calls `open_suggest_feature_dialog(None)` (or routes through the app's window-getter the way "Settings" does).

- [ ] **Step 6: Smoke-run** (requires a Linux GTK4 environment — if the dev box is macOS, this is verified in CI or by the user on Linux hardware). Launch the app, open Settings → "Suggest a feature" → "Open…", fill the form, submit against a local backend (`backend_url` configured). Expect the success toast and a new `kind=feature` row. If no Linux environment is available, at minimum run `python -m py_compile draftright/services/feedback_service.py draftright/ui/suggest_feature_dialog.py` to catch syntax errors, and note that runtime verification is deferred.

- [ ] **Step 7: Commit**

```bash
git add DraftRightLinux/draftright/services/feedback_service.py DraftRightLinux/draftright/ui/suggest_feature_dialog.py DraftRightLinux/draftright/ui/settings_window.py DraftRightLinux/draftright/ui/tray_icon.py
git commit -m "feat(linux): 'Suggest a feature' dialog (Settings + tray) → POST /feedback"
```

---

### Task 6: Docs + changelog

**Files:**
- Modify: `docs/changelog.md` (add a bullet under `## 2026-05-12` — or a new `## 2026-05-13` section if that's the convention by the time this runs)
- Modify: `website/CLAUDE.md`, `DraftRight/CLAUDE.md`, `DraftRightWindows/CLAUDE.md` (or wherever each subdir documents its UI surfaces — check), `DraftRightMobile/CLAUDE.md`, `DraftRightLinux/CLAUDE.md` — one line each noting the new "Suggest a feature" surface and that it posts to `POST /feedback`.

- [ ] **Step 1: Changelog** — add:

```markdown
### Feedback / feature-request client surfaces (Spec B)
- "Suggest a feature" form (title + target-platform dropdown + description) added to: web playground (`SuggestFeatureWidget`), macOS (menu bar + Advanced settings), Windows (Settings → Feedback), Flutter iOS/Android (Settings → Help), Linux (Settings + tray). All POST JSON `{kind:"feature", title, target_platform, description, source}` to `/feedback` with the user's Bearer token when signed in; each has a "See all requests →" link to `draftright.info/feedback`. Backend = Spec A; public board = Spec C (pending).
```

- [ ] **Step 2: Subdir CLAUDE.md updates** — add a one-liner to each client's CLAUDE.md (matching its existing format) describing the new surface and the `feedback_service`/`FeedbackService` it uses.

- [ ] **Step 3: Commit**

```bash
git add docs/changelog.md '**/CLAUDE.md'
git commit -m "docs(feedback): document Spec B client 'Suggest a feature' surfaces"
```

---

## Self-review notes (for the executor)

- **Spec coverage:** every client from the design spec's "Clients" table has a task (web/macOS/Windows/Linux/iOS+Android). Each sends `kind:"feature"` + `title` + the user-picked `target_platform` + `description` + `source` + optional `user_email`, with the Bearer JWT when signed in (Q4: login encouraged, not required for submit per the backend — clients send the token if they have one, email if not). Each has the "See all requests →" deep-link to `https://draftright.info/feedback` (Q3).
- **Title length:** backend accepts 1–80 chars (Spec A shipped 1-80, not 3-80). Clients only check non-empty + `maxLength 80`; the backend is the authority.
- **No screenshots** on feature requests — clients post JSON, not multipart (the legacy `POST /bug-reports` multipart route is for bugs only).
- **Platform note:** Windows and Linux can't be built/run on the macOS dev box — those tasks rely on CI / the user's hardware for runtime verification; the plan calls this out in their build/smoke steps. Tasks 1 (web), 2 (macOS), 4 (Flutter) are fully verifiable locally.
- **`source` values** intentionally differ from the existing bug-report `source` tags (`macos`/`windows`/`web-playground`/`marketing-site`) — the feedback design spec specifies `macos-app`/`windows-app`/`linux-app`/`web` and these are what the admin board will filter on. Keep them as written.

## After this plan

Spec B done — users can submit feature requests from every client. Final piece: **Spec C** — the public `draftright.info/feedback` Astro board page (card list with vote buttons + platform/status filters + on-page submit form). Then deploy: apply `backend/sql/2026-05-12-feedback.sql` to prod and ship `develop` → testing → `main` across all clients.
