using System.Collections.Generic;
using System.Diagnostics;
using System.Drawing;
using DraftRightWindows.Helpers;
using DraftRightWindows.Models;
using DraftRightWindows.Services;
using DraftRightWindows.Views;
using WinForms = System.Windows.Forms;

namespace DraftRightWindows;

// ── Tabbed WinForms Settings Form builder ──
internal static class SettingsFormBuilder
{
    // ── Dark theme constants ─────────────────────────────────
    private static readonly Color BgDark = Color.FromArgb(15, 23, 42);
    private static readonly Color CardBg = Color.FromArgb(30, 41, 59);
    private static readonly Color BrandBlue = Color.FromArgb(93, 135, 255);
    private static readonly Color TextPrimary = Color.FromArgb(226, 232, 240);
    private static readonly Color TextMuted = Color.FromArgb(148, 163, 184);
    private static readonly Color ErrorRed = Color.FromArgb(239, 68, 68);
    private static readonly Color SuccessGreen = Color.FromArgb(34, 197, 94);
    private static readonly Color BorderColor = Color.FromArgb(51, 65, 85);

    public static WinForms.Form Create()
    {
        var form = new WinForms.Form
        {
            Text = "DraftRight Settings",
            Width = 520,
            Height = 560,
            StartPosition = WinForms.FormStartPosition.CenterScreen,
            BackColor = BgDark,
            ForeColor = TextPrimary,
            FormBorderStyle = WinForms.FormBorderStyle.FixedSingle,
            MaximizeBox = false,
        };

        SetFormIcon(form);

        var tabControl = new WinForms.TabControl
        {
            Dock = WinForms.DockStyle.Fill,
            BackColor = BgDark,
            ForeColor = TextPrimary,
            Appearance = WinForms.TabAppearance.Normal,
            SizeMode = WinForms.TabSizeMode.FillToRight,
            Padding = new System.Drawing.Point(12, 6),
        };

        tabControl.TabPages.Add(BuildGeneralTab());
        tabControl.TabPages.Add(BuildRewriteTab());
        tabControl.TabPages.Add(BuildTriggerTab());
        tabControl.TabPages.Add(BuildAccountTab());
        tabControl.TabPages.Add(BuildSubscriptionTab());
        tabControl.TabPages.Add(BuildAdvancedTab());

        form.Controls.Add(tabControl);
        return form;
    }

    // ── Helpers ──────────────────────────────────────────────

    /// <summary>
    /// Permissive email check used to short-circuit obviously-invalid input
    /// before the API call. Backend validation is still authoritative — this
    /// just spares the user a 400 round-trip + stack-trace popup.
    /// </summary>
    private static bool IsValidEmail(string s)
    {
        if (string.IsNullOrWhiteSpace(s)) return false;
        var at = s.IndexOf('@');
        if (at <= 0 || at == s.Length - 1) return false;
        return s.IndexOf('.', at) > at;
    }

    private static void SetFormIcon(WinForms.Form form)
    {
        var exePath = Environment.ProcessPath;
        if (exePath != null)
        {
            var icoPath = System.IO.Path.Combine(
                System.IO.Path.GetDirectoryName(exePath)!, "Assets", "DraftRight.ico");
            if (System.IO.File.Exists(icoPath))
                form.Icon = new Icon(icoPath);
        }
    }

    private static WinForms.TabPage MakeTab(string title)
    {
        return new WinForms.TabPage(title)
        {
            BackColor = BgDark,
            ForeColor = TextPrimary,
            Padding = new WinForms.Padding(16),
            AutoScroll = true,
        };
    }

    private static WinForms.Label MakeSectionHeader(string text, int y)
    {
        return new WinForms.Label
        {
            Text = text,
            Font = new Font("Segoe UI", 12, FontStyle.Bold),
            ForeColor = TextPrimary,
            Location = new Point(16, y),
            AutoSize = true,
        };
    }

    private static WinForms.Label MakeFieldLabel(string text, int y)
    {
        return new WinForms.Label
        {
            Text = text,
            Font = new Font("Segoe UI", 9),
            ForeColor = TextMuted,
            Location = new Point(16, y),
            AutoSize = true,
        };
    }

    private static WinForms.TextBox MakeTextBox(int y, string value = "")
    {
        return new WinForms.TextBox
        {
            Text = value,
            Location = new Point(16, y),
            Size = new Size(448, 30),
            BackColor = CardBg,
            ForeColor = TextPrimary,
            BorderStyle = WinForms.BorderStyle.FixedSingle,
            Font = new Font("Segoe UI", 10),
        };
    }

    private static WinForms.ComboBox MakeComboBox(int y)
    {
        return new WinForms.ComboBox
        {
            Location = new Point(16, y),
            Size = new Size(448, 30),
            BackColor = CardBg,
            ForeColor = TextPrimary,
            Font = new Font("Segoe UI", 10),
            DropDownStyle = WinForms.ComboBoxStyle.DropDownList,
            FlatStyle = WinForms.FlatStyle.Flat,
        };
    }

    private static WinForms.Button MakePrimaryButton(string text, int y, int width = 448)
    {
        var btn = new WinForms.Button
        {
            Text = text,
            Location = new Point(16, y),
            Size = new Size(width, 36),
            BackColor = BrandBlue,
            ForeColor = Color.White,
            FlatStyle = WinForms.FlatStyle.Flat,
            Font = new Font("Segoe UI", 10, FontStyle.Bold),
        };
        btn.FlatAppearance.BorderSize = 0;
        return btn;
    }

    private static WinForms.Button MakeSecondaryButton(string text, int x, int y, int width = 160)
    {
        var btn = new WinForms.Button
        {
            Text = text,
            Location = new Point(x, y),
            Size = new Size(width, 32),
            BackColor = CardBg,
            ForeColor = TextPrimary,
            FlatStyle = WinForms.FlatStyle.Flat,
            Font = new Font("Segoe UI", 10),
        };
        btn.FlatAppearance.BorderColor = BorderColor;
        return btn;
    }

    private static WinForms.CheckBox MakeCheckBox(string text, bool checkedInit, int y)
    {
        return new WinForms.CheckBox
        {
            Text = text,
            Checked = checkedInit,
            ForeColor = TextPrimary,
            BackColor = BgDark,
            Font = new Font("Segoe UI", 10),
            Location = new Point(16, y),
            AutoSize = true,
        };
    }

    // ── Tab builders ─────────────────────────────────────────

    private static WinForms.TabPage BuildGeneralTab()
    {
        var tab = MakeTab("General");
        int y = 16;

        // Backend URL is no longer user-editable from the Settings UI — see
        // Constants.DefaultBackendUrl + AppSettings (env var / settings file
        // override path for self-hosting developers).

        // General section
        tab.Controls.Add(MakeSectionHeader("General", y));
        y += 30;
        // Reflect the actual registry state (the installer's autostart task or a
        // prior toggle may have set it), not just the saved bool.
        var autoStart = MakeCheckBox("Launch at Login", StartupRegistration.IsEnabled(), y);
        autoStart.CheckedChanged += (_, _) =>
        {
            StartupRegistration.SetEnabled(autoStart.Checked);
            App.Settings.AutoStart = autoStart.Checked;
            App.Settings.Save();
            // Mirror the toggle in the Scheduled Task that supervises the
            // running process. Toggle ON installs (logon launch + crash
            // respawn). Toggle OFF removes the task entirely.
            Services.KeepAliveAgent.Reconcile(desiredRunAtLogon: autoStart.Checked);
        };
        tab.Controls.Add(autoStart);
        y += 34;

        // Updates section
        tab.Controls.Add(MakeSectionHeader("Updates", y));
        y += 30;
        var asmVer = System.Reflection.Assembly.GetExecutingAssembly().GetName().Version?.ToString() ?? "?";
        var displayVer = asmVer.EndsWith(".0") ? asmVer.Substring(0, asmVer.Length - 2) : asmVer;
        tab.Controls.Add(new WinForms.Label
        {
            Text = $"Version: {displayVer}",
            ForeColor = TextMuted,
            Font = new Font("Segoe UI", 9),
            Location = new Point(16, y),
            AutoSize = true,
        });
        y += 24;

        // "Update X.Y.Z available — click here to download and install" link.
        // Hidden when up to date. Driven by UpdateService.AvailableUpdate so
        // it reflects the background check without the user pressing the
        // button below.
        var updateLink = new WinForms.LinkLabel
        {
            AutoSize = true,
            Location = new Point(16, y),
            Font = new Font("Segoe UI", 9, FontStyle.Bold),
            LinkColor = Color.FromArgb(96, 165, 250),
            ActiveLinkColor = Color.FromArgb(147, 197, 253),
            Visible = false,
        };
        updateLink.LinkClicked += (_, _) =>
        {
            var u = App.UpdateService?.AvailableUpdate;
            if (u != null) App.UpdateService!.StartInstall(u);
        };
        tab.Controls.Add(updateLink);
        y += 26;

        void RefreshUpdateLink()
        {
            var u = App.UpdateService?.AvailableUpdate;
            if (u != null)
            {
                var staged = App.UpdateService?.UpdateStaged ?? false;
                updateLink.Text = staged
                    ? $"Update {u.Version} downloaded — click here to restart and install"
                    : $"Update {u.Version} available — click here to download and install";
                updateLink.Visible = true;
            }
            else
            {
                updateLink.Visible = false;
            }
        }
        RefreshUpdateLink();
        // Re-poll in the background so a newly-published update appears here
        // even if the 10s-after-launch check ran before it went live.
        if (App.UpdateService != null)
        {
            _ = App.UpdateService.RefreshAvailableUpdateAsync().ContinueWith(_ =>
            {
                try
                {
                    if (!updateLink.IsDisposed)
                        updateLink.BeginInvoke(new Action(RefreshUpdateLink));
                }
                catch { /* settings window closed */ }
            });
        }

        var updateBtn = MakeSecondaryButton("Check for Updates", 16, y, 180);
        updateBtn.Click += async (_, _) =>
        {
            updateBtn.Enabled = false;
            updateBtn.Text = "Checking...";
            if (App.UpdateService != null)
                await App.UpdateService.CheckNowAsync();
            RefreshUpdateLink();
            updateBtn.Text = "Check for Updates";
            updateBtn.Enabled = true;
        };
        tab.Controls.Add(updateBtn);

        return tab;
    }

    private static WinForms.TabPage BuildRewriteTab()
    {
        var tab = MakeTab("Rewrite");
        int y = 16;
        var allTones = Enum.GetValues<Tone>();

        // ── Mode section (always visible) ───────────────────
        tab.Controls.Add(MakeSectionHeader("Mode", y));
        y += 30;
        tab.Controls.Add(MakeFieldLabel("Interaction Mode", y));
        y += 18;
        var modeCombo = MakeComboBox(y);
        modeCombo.Items.Add(AppMode.Advanced.DisplayName());
        modeCombo.Items.Add(AppMode.OneClick.DisplayName());
        modeCombo.SelectedIndex = App.Settings.AppMode == AppMode.OneClick ? 1 : 0;
        tab.Controls.Add(modeCombo);
        y += 44;

        // ── Simple + Advanced blocks share the same starting Y ──
        // Only one block is visible at a time; the other is hidden so the
        // user never sees an empty gap between Mode and the visible block.
        int conditionalY = y;

        var simpleOnlyControls = new List<WinForms.Control>();
        var advancedOnlyControls = new List<WinForms.Control>();

        // ── Simple block: Simple Tone + Default Tone ────────
        int sy = conditionalY;
        var oneClickLabel = MakeFieldLabel($"{AppMode.OneClick.DisplayName()} Tone", sy);
        tab.Controls.Add(oneClickLabel); simpleOnlyControls.Add(oneClickLabel);
        sy += 18;
        var oneClickCombo = MakeComboBox(sy);
        foreach (var t in allTones) oneClickCombo.Items.Add(t.DisplayName());
        int initialIdx = 0;
        for (int i = 0; i < allTones.Length; i++)
        {
            if (allTones[i].ApiValue() == App.Settings.OneClickTone)
            {
                initialIdx = i;
                break;
            }
        }
        oneClickCombo.SelectedIndex = initialIdx;
        oneClickCombo.SelectedIndexChanged += (_, _) =>
        {
            if (oneClickCombo.SelectedIndex >= 0 && oneClickCombo.SelectedIndex < allTones.Length)
            {
                App.Settings.OneClickTone = allTones[oneClickCombo.SelectedIndex].ApiValue();
                App.Settings.Save();
            }
        };
        tab.Controls.Add(oneClickCombo); simpleOnlyControls.Add(oneClickCombo);
        sy += 44;
        int simpleBlockHeight = sy - conditionalY;

        // ── Advanced block: Panel Tones ─────────────────────
        int ay = conditionalY;
        var panelTonesHeader = MakeSectionHeader("Panel Tones", ay);
        tab.Controls.Add(panelTonesHeader); advancedOnlyControls.Add(panelTonesHeader);
        ay += 30;
        foreach (var tone in allTones)
        {
            var cb = MakeCheckBox(
                $"{tone.Icon()}  {tone.DisplayName()}",
                App.Settings.EnabledTones.Contains(tone.ApiValue()),
                ay);
            cb.CheckedChanged += (_, _) =>
            {
                var apiVal = tone.ApiValue();
                if (cb.Checked)
                {
                    if (!App.Settings.EnabledTones.Contains(apiVal))
                        App.Settings.EnabledTones.Add(apiVal);
                }
                else
                {
                    App.Settings.EnabledTones.Remove(apiVal);
                }
                App.Settings.Save();
            };
            tab.Controls.Add(cb); advancedOnlyControls.Add(cb);
            ay += 26;
        }

        // Default Tone (auto-run): only meaningful in Advanced mode — when the
        // rewrite panel opens it runs this tone immediately (empty = manual
        // pick). Lives in the Advanced block so it's visible where it applies.
        ay += 8;
        var defaultToneLabel = MakeFieldLabel("Default Tone (auto-run)", ay);
        tab.Controls.Add(defaultToneLabel); advancedOnlyControls.Add(defaultToneLabel);
        ay += 18;
        var defaultCombo = MakeComboBox(ay);
        int defaultSelected = 0;
        for (int i = 0; i < allTones.Length; i++)
        {
            defaultCombo.Items.Add(allTones[i].DisplayName());
            if (allTones[i].ApiValue() == App.Settings.DefaultTone)
                defaultSelected = i;
        }
        defaultCombo.SelectedIndex = defaultSelected;
        defaultCombo.SelectedIndexChanged += (_, _) =>
        {
            if (defaultCombo.SelectedIndex >= 0 && defaultCombo.SelectedIndex < allTones.Length)
            {
                App.Settings.DefaultTone = allTones[defaultCombo.SelectedIndex].ApiValue();
                App.Settings.Save();
            }
        };
        tab.Controls.Add(defaultCombo); advancedOnlyControls.Add(defaultCombo);
        ay += 30;
        int advancedBlockHeight = ay - conditionalY;

        // ── Translation section (always visible, position depends on mode) ──
        // Constructed at y=0 then re-positioned by UpdateModeVisibility so it
        // always sits directly below the visible block — no gaps, no overlap.
        var translationHeader = MakeSectionHeader("Translation", 0);
        var translationLabel = MakeFieldLabel("Target Language", 0);

        // Editable ComboBox: pre-populated with common languages but the user
        // can type any value for niche ones — the backend passes the string
        // straight to the AI, so anything human-readable works.
        var langBox = new WinForms.ComboBox
        {
            Location = new Point(16, 0),
            Size = new Size(448, 30),
            BackColor = CardBg,
            ForeColor = TextPrimary,
            Font = new Font("Segoe UI", 10),
            DropDownStyle = WinForms.ComboBoxStyle.DropDown,
            FlatStyle = WinForms.FlatStyle.Flat,
            AutoCompleteMode = WinForms.AutoCompleteMode.SuggestAppend,
            AutoCompleteSource = WinForms.AutoCompleteSource.ListItems,
        };
        string[] commonLanguages =
        {
            "English", "Vietnamese", "Spanish", "French", "German", "Italian",
            "Portuguese", "Dutch", "Russian", "Japanese", "Korean",
            "Chinese (Simplified)", "Chinese (Traditional)", "Arabic", "Hindi",
            "Thai", "Indonesian", "Turkish", "Polish",
        };
        langBox.Items.AddRange(commonLanguages);
        langBox.Text = App.Settings.TranslateLanguage ?? "";
        langBox.TextChanged += (_, _) =>
        {
            App.Settings.TranslateLanguage = langBox.Text.Trim();
            App.Settings.Save();
        };
        tab.Controls.Add(translationHeader);
        tab.Controls.Add(translationLabel);
        tab.Controls.Add(langBox);

        void UpdateModeVisibility()
        {
            bool isSimple = modeCombo.SelectedIndex == 1;
            foreach (var c in simpleOnlyControls) c.Visible = isSimple;
            foreach (var c in advancedOnlyControls) c.Visible = !isSimple;

            int visibleHeight = isSimple ? simpleBlockHeight : advancedBlockHeight;
            int translationY = conditionalY + visibleHeight + 14;
            translationHeader.Top = translationY;
            translationLabel.Top  = translationY + 30;
            langBox.Top           = translationY + 48;
        }
        UpdateModeVisibility();

        modeCombo.SelectedIndexChanged += (_, _) =>
        {
            App.Settings.AppMode = modeCombo.SelectedIndex == 1 ? AppMode.OneClick : AppMode.Advanced;
            App.Settings.Save();
            UpdateModeVisibility();
        };

        return tab;
    }

    private static WinForms.TabPage BuildTriggerTab()
    {
        var tab = MakeTab("Trigger");
        int y = 16;

        tab.Controls.Add(MakeSectionHeader("Hotkey", y));
        y += 30;

        tab.Controls.Add(MakeFieldLabel("Current Hotkey", y));
        y += 18;

        tab.Controls.Add(new WinForms.Label
        {
            Text = FormatHotkey(App.Settings.HotkeyModifiers, App.Settings.HotkeyKey),
            ForeColor = BrandBlue,
            Font = new Font("Segoe UI", 10, FontStyle.Bold),
            Location = new Point(16, y),
            AutoSize = true,
        });
        y += 32;

        tab.Controls.Add(new WinForms.Label
        {
            Text = "Select text anywhere, then press the hotkey to rewrite.\r\n" +
                   "Hotkey editing is not yet available in this build.",
            ForeColor = TextMuted,
            Font = new Font("Segoe UI", 9),
            Location = new Point(16, y),
            Size = new Size(448, 40),
        });

        return tab;
    }

    private static string FormatHotkey(int mods, int key)
    {
        var parts = new List<string>();
        if ((mods & 0x0002) != 0) parts.Add("Ctrl");
        if ((mods & 0x0004) != 0) parts.Add("Shift");
        if ((mods & 0x0001) != 0) parts.Add("Alt");
        if ((mods & 0x0008) != 0) parts.Add("Win");
        var keyChar = key >= 0x41 && key <= 0x5A
            ? ((char)key).ToString()
            : $"Key0x{key:X2}";
        parts.Add(keyChar);
        return string.Join(" + ", parts);
    }

    private static WinForms.TabPage BuildAccountTab()
    {
        var tab = MakeTab("Account");
        PopulateAccountTab(tab);
        return tab;
    }

    /// <summary>
    /// Hosts <see cref="SubscriptionTab"/> inside a Settings TabPage so
    /// the user can manage their plan without leaving the Settings
    /// window.  Logic lives in the UserControl; this method is just the
    /// wiring.
    /// </summary>
    private static WinForms.TabPage BuildSubscriptionTab()
    {
        var tab = MakeTab("Subscription");
        tab.Padding = new WinForms.Padding(0); // SubscriptionTab handles its own padding
        tab.Controls.Add(new SubscriptionTab());
        return tab;
    }

    /// <summary>
    /// Renders the Account tab body based on current auth state. Called once
    /// at tab construction, then again whenever the user signs in or out so
    /// the UI flips between Sign In and Signed In without requiring the
    /// Settings window to be reopened.
    /// </summary>
    private static void PopulateAccountTab(WinForms.TabPage tab)
    {
        // Tear down existing controls before re-rendering. Dispose explicitly so
        // event handlers don't keep references to disposed boxes/buttons alive.
        var existing = new System.Collections.Generic.List<WinForms.Control>();
        foreach (WinForms.Control c in tab.Controls) existing.Add(c);
        tab.Controls.Clear();
        foreach (var c in existing) c.Dispose();

        int y = 16;
        var signedIn = !string.IsNullOrEmpty(App.Auth.AccessToken);

        if (signedIn)
        {
            tab.Controls.Add(MakeSectionHeader("Signed In", y));
            y += 30;

            var emailDisplay = App.Auth.CurrentEmail;
            tab.Controls.Add(new WinForms.Label
            {
                Text = string.IsNullOrEmpty(emailDisplay)
                    ? "Signed in"
                    : $"Signed in as {emailDisplay}",
                ForeColor = SuccessGreen,
                Font = new Font("Segoe UI", 10),
                Location = new Point(16, y),
                AutoSize = true,
            });
            y += 32;

            var signOutBtn = MakeSecondaryButton("Sign Out", 16, y);
            signOutBtn.Click += (_, _) =>
            {
                App.Auth.ClearTokens();
                App.Api.ClearToken();
                PopulateAccountTab(tab);
            };
            tab.Controls.Add(signOutBtn);
        }
        else
        {
            tab.Controls.Add(MakeSectionHeader("Sign In", y));
            y += 30;

            tab.Controls.Add(MakeFieldLabel("Email", y));
            y += 18;
            var emailBox = MakeTextBox(y);
            tab.Controls.Add(emailBox);
            y += 44;

            tab.Controls.Add(MakeFieldLabel("Password", y));
            y += 18;
            var passBox = MakeTextBox(y);
            passBox.UseSystemPasswordChar = true;
            tab.Controls.Add(passBox);
            y += 44;

            // Read-only multi-line TextBox styled as a label so the user
            // can select + copy any error message (Ctrl+C, right-click, drag).
            // WinForms.Label.Text isn't selectable; TextBox.Text is.
            var statusBox = new WinForms.TextBox
            {
                ReadOnly = true,
                Multiline = true,
                ForeColor = ErrorRed,
                BackColor = BgDark,
                BorderStyle = WinForms.BorderStyle.None,
                Font = new Font("Segoe UI", 9),
                Location = new Point(16, y),
                Size = new Size(380, 60),
                ScrollBars = WinForms.ScrollBars.Vertical,
                TabStop = false,
                Visible = false,  // hidden until there's a message
            };
            tab.Controls.Add(statusBox);

            // One-click Copy button — hidden until there's text to copy.
            var copyBtn = new WinForms.Button
            {
                Text = "Copy",
                Location = new Point(400, y),
                Size = new Size(64, 24),
                BackColor = CardBg,
                ForeColor = TextMuted,
                FlatStyle = WinForms.FlatStyle.Flat,
                Font = new Font("Segoe UI", 8),
                Visible = false,
                TabStop = false,
            };
            copyBtn.FlatAppearance.BorderColor = BorderColor;
            copyBtn.Click += (_, _) =>
            {
                if (!string.IsNullOrEmpty(statusBox.Text))
                {
                    WinForms.Clipboard.SetText(statusBox.Text);
                    copyBtn.Text = "Copied";
                    var t = new WinForms.Timer { Interval = 1200 };
                    t.Tick += (_, _) => { copyBtn.Text = "Copy"; t.Stop(); t.Dispose(); };
                    t.Start();
                }
            };
            tab.Controls.Add(copyBtn);

            // Helper: show/hide the status box + copy button together.
            Action<string, Color> setStatus = (text, color) =>
            {
                statusBox.Text = text;
                statusBox.ForeColor = color;
                var hasText = !string.IsNullOrEmpty(text);
                statusBox.Visible = hasText;
                // Only show Copy for actual errors (red), not success messages.
                copyBtn.Visible = hasText && color == ErrorRed;
            };

            y += 72;

            var signInBtn = MakePrimaryButton("Sign In", y);
            signInBtn.Click += async (_, _) =>
            {
                setStatus("", ErrorRed);

                // Client-side validation BEFORE hitting /auth/login so the
                // backend's 400 ("email must be an email") doesn't bubble up
                // as a raw ApiException stack trace (BUG-18 / BUG-19).
                var email = emailBox.Text.Trim();
                var password = passBox.Text;
                if (string.IsNullOrEmpty(email))
                {
                    setStatus("Please enter your email.", ErrorRed);
                    return;
                }
                if (!IsValidEmail(email))
                {
                    setStatus("Please enter a valid email address.", ErrorRed);
                    return;
                }
                if (string.IsNullOrEmpty(password))
                {
                    setStatus("Please enter your password.", ErrorRed);
                    return;
                }

                signInBtn.Enabled = false;
                try
                {
                    App.Api.SetBaseUrl(App.Settings.BackendUrl ?? "");
                    var result = await App.Api.LoginAsync(email, password);
                    if (!string.IsNullOrEmpty(result.AccessToken))
                    {
                        App.Auth.SaveTokens(result.AccessToken, result.RefreshToken, result.User?.Email);
                        App.Api.SetToken(result.AccessToken);
                        // Flip the tab to the Signed In view immediately.
                        PopulateAccountTab(tab);
                    }
                    else
                    {
                        setStatus("Login failed.", ErrorRed);
                    }
                }
                catch (ApiException apiEx)
                {
                    // Show the parsed server message ("Invalid credentials",
                    // "email must be an email", etc.) — NOT the full stack
                    // trace, which is what users were seeing before.
                    setStatus(apiEx.ServerMessage ?? apiEx.Message, ErrorRed);
                    DRLogger.Error($"Login API error: {apiEx}", DRLogger.Category.AUTH);
                }
                catch (Exception ex)
                {
                    setStatus("Something went wrong. Please try again.", ErrorRed);
                    DRLogger.Error($"Login error: {ex}", DRLogger.Category.AUTH);
                }
                finally
                {
                    // signInBtn may have been disposed if PopulateAccountTab ran.
                    try { signInBtn.Enabled = true; } catch (ObjectDisposedException) { }
                }
            };
            tab.Controls.Add(signInBtn);

            // "Continue with Google" — single button covers BOTH sign-in and
            // sign-up: the backend's /auth/social endpoint creates the user
            // on first call and signs in existing users on every call after,
            // so a separate Register screen isn't needed on Windows.
            // Public Desktop OAuth client + PKCE in
            // GoogleOAuth.AuthenticateAsync exchanges the id_token at
            // /auth/social for a DraftRight session, exactly like the
            // iOS/Android/macOS clients.
            var googleBtn = MakeSecondaryButton("Continue with Google", 16, y + 44, 448);
            googleBtn.Click += async (_, _) =>
            {
                setStatus("", ErrorRed);
                googleBtn.Enabled = false;
                signInBtn.Enabled = false;
                try
                {
                    App.Api.SetBaseUrl(App.Settings.BackendUrl ?? "");
                    var idToken = await GoogleOAuth.AuthenticateAsync();
                    var result = await App.Api.SocialLoginAsync("google", idToken);
                    if (!string.IsNullOrEmpty(result.AccessToken))
                    {
                        App.Auth.SaveTokens(result.AccessToken, result.RefreshToken, result.User?.Email);
                        App.Api.SetToken(result.AccessToken);
                        PopulateAccountTab(tab);
                    }
                    else
                    {
                        setStatus("Google sign-in failed.", ErrorRed);
                    }
                }
                catch (ApiException apiEx)
                {
                    setStatus(apiEx.ServerMessage ?? apiEx.Message, ErrorRed);
                    DRLogger.Error($"Google sign-in API error: {apiEx}", DRLogger.Category.AUTH);
                }
                catch (Exception ex)
                {
                    setStatus(ex.Message, ErrorRed);
                    DRLogger.Error($"Google sign-in error: {ex}", DRLogger.Category.AUTH);
                }
                finally
                {
                    try { googleBtn.Enabled = true; signInBtn.Enabled = true; }
                    catch (ObjectDisposedException) { }
                }
            };
            tab.Controls.Add(googleBtn);

            // Communicates that the Google button is also the sign-up path,
            // since there's no separate Register screen on Windows.
            var googleHint = new WinForms.Label
            {
                Text = "New to DraftRight? Use \"Continue with Google\" to create your account in seconds.",
                Font = new Font("Segoe UI", 8.5f),
                ForeColor = TextMuted,
                Location = new Point(16, y + 80),
                AutoSize = true,
            };
            tab.Controls.Add(googleHint);
        }
    }

    private static WinForms.TabPage BuildAdvancedTab()
    {
        var tab = MakeTab("Advanced");
        int y = 16;

        tab.Controls.Add(MakeSectionHeader("Logs", y));
        y += 30;

        var loggingEnabled = MakeCheckBox("Enable Logging", App.Settings.LoggingEnabled, y);
        loggingEnabled.CheckedChanged += (_, _) =>
        {
            App.Settings.LoggingEnabled = loggingEnabled.Checked;
            App.Settings.Save();
            DRLogger.IsEnabled = loggingEnabled.Checked;
        };
        tab.Controls.Add(loggingEnabled);
        y += 34;

        tab.Controls.Add(MakeFieldLabel("Log file location:", y));
        y += 20;

        var logFilePath = DRLogger.LogFilePath;
        var pathBox = MakeTextBox(y, logFilePath);
        pathBox.ReadOnly = true;
        tab.Controls.Add(pathBox);
        y += 44;

        var openBtn = MakeSecondaryButton("Open", 16, y, 80);
        openBtn.Click += (_, _) =>
        {
            if (System.IO.File.Exists(logFilePath))
            {
                System.Diagnostics.Process.Start(
                    new System.Diagnostics.ProcessStartInfo(logFilePath) { UseShellExecute = true });
            }
            else
            {
                WinForms.MessageBox.Show(
                    "Log file does not exist yet.",
                    "DraftRight",
                    WinForms.MessageBoxButtons.OK,
                    WinForms.MessageBoxIcon.Information);
            }
        };
        tab.Controls.Add(openBtn);

        var clearBtn = MakeSecondaryButton("Clear", 108, y, 80);
        clearBtn.Click += (_, _) =>
        {
            try
            {
                System.IO.File.WriteAllText(logFilePath, "");
            }
            catch (Exception ex)
            {
                WinForms.MessageBox.Show(
                    $"Could not clear log: {ex.Message}",
                    "DraftRight",
                    WinForms.MessageBoxButtons.OK,
                    WinForms.MessageBoxIcon.Warning);
            }
        };
        tab.Controls.Add(clearBtn);
        y += 50;

        // Feedback section — opens the bug-report dialog (mirrors macOS).
        tab.Controls.Add(MakeSectionHeader("Feedback", y));
        y += 30;
        tab.Controls.Add(new WinForms.Label
        {
            Text = "Hit a bug? Send us a description (and a screenshot if you have one) and we'll take a look.",
            ForeColor = TextMuted,
            Font = new Font("Segoe UI", 9),
            Location = new Point(16, y),
            Size = new Size(448, 36),
        });
        y += 40;
        var bugBtn = MakeSecondaryButton("Report a Bug…", 16, y, 160);
        bugBtn.Click += (_, _) => Views.ReportBugDialog.Show();
        tab.Controls.Add(bugBtn);

        var featureBtn = MakeSecondaryButton("Suggest a feature…", 188, y, 180);
        featureBtn.Click += (_, _) => Views.SuggestFeatureDialog.Show();
        tab.Controls.Add(featureBtn);

        return tab;
    }
}
