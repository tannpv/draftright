using System;
using System.Collections.Generic;
using System.Threading;
using System.Windows;
using System.Windows.Controls;
using System.Windows.Threading;
using DraftRightWindows.Models;
using DraftRightWindows.Services;
using Wpf.Ui.Controls;
using WpfButton = Wpf.Ui.Controls.Button;
using WpfTextBox = Wpf.Ui.Controls.TextBox;
using WpfTextBlock = System.Windows.Controls.TextBlock;
using F = DraftRightWindows.Views.FluentFormControls;

namespace DraftRightWindows.Views;

/// <summary>
/// The Settings window in WPF UI (#159, phase 3 of #156) — six tabs: General,
/// Rewrite, Trigger, Account, Subscription, Advanced. Replaces the WinForms
/// SettingsFormBuilder + hosts the WPF <see cref="SubscriptionTab"/>. Derives
/// <see cref="FluentWindowBase"/> for the theme contract; every field comes from
/// <see cref="FluentFormControls"/> so the six tabs share one vocabulary and
/// WPF layout panels replace the per-tab pixel coordinates.
/// </summary>
internal sealed class SettingsWindow : FluentWindowBase
{
    private static readonly object _lock = new();
    private static SettingsWindow? _instance;

    /// <summary>
    /// Opens Settings on its own STA thread (same pattern as the rewrite panel
    /// and bug dialog). If already open, brings the existing window forward
    /// instead of stacking a second one.
    /// </summary>
    public static void Open()
    {
        lock (_lock)
        {
            if (_instance != null)
            {
                try { _instance.Dispatcher.BeginInvoke(new Action(() => { _instance!.Activate(); })); return; }
                catch { _instance = null; }
            }
        }

        Services.StaWindowHost.Run("settings", () =>
        {
            var win = new SettingsWindow();
            lock (_lock) { _instance = win; }
            // Clear the singleton on close; StaWindowHost adds the dispatcher shutdown.
            win.Closed += (_, _) => { lock (_lock) { if (ReferenceEquals(_instance, win)) _instance = null; } };
            return win;
        });
    }

    /// <summary>Closes the window if open (used on app quit).</summary>
    public static void CloseIfOpen()
    {
        lock (_lock)
        {
            var w = _instance;
            if (w == null) return;
            try { w.Dispatcher.BeginInvoke(new Action(w.Close)); } catch { /* already gone */ }
        }
    }

    private SettingsWindow()
    {
        Title = "DraftRight Settings";
        // Keep the window comfortably INSIDE the work area, with real slack, and
        // centre it within the work area (not the full screen). On a scaled
        // display the work area is small in DIPs — e.g. 1080p @ 150% ≈ 680 DIP
        // tall — so a 680 window would fill it and shove the title bar to the
        // edge with no way to close it (#165 recurred). Leave >=80px vertical /
        // >=40px horizontal margin, and position by the work area so the whole
        // window (title bar included) is always on-screen. Stays resizable.
        var work = SystemParameters.WorkArea;
        double maxH = Math.Max(360, work.Height - 80);
        double maxW = Math.Max(420, work.Width - 40);
        Height = Math.Min(680, maxH);
        Width = Math.Min(660, maxW);
        MinHeight = Math.Min(440, maxH);
        MinWidth = Math.Min(480, maxW);
        MaxHeight = work.Height;
        MaxWidth = work.Width;
        WindowStartupLocation = WindowStartupLocation.Manual;
        Left = work.Left + (work.Width - Width) / 2;
        Top = work.Top + (work.Height - Height) / 2;

        var icon = Helpers.AppIcon.LoadImageSource();
        if (icon != null) Icon = icon;

        var tabs = new TabControl { Margin = new Thickness(0) };
        tabs.Items.Add(Tab("General", BuildGeneral()));
        tabs.Items.Add(Tab("Rewrite", BuildRewrite()));
        tabs.Items.Add(Tab("Trigger", BuildTrigger()));
        tabs.Items.Add(Tab("Account", BuildAccount()));
        tabs.Items.Add(Tab("Subscription", new SubscriptionTab()));
        tabs.Items.Add(Tab("Advanced", BuildAdvanced()));
        SetBody(tabs, showMaximize: true); // resizable window
    }

    // Each tab: a scrollable padded stack.
    private static TabItem Tab(string header, UIElement content) => new()
    {
        Header = header,
        Content = new ScrollViewer
        {
            VerticalScrollBarVisibility = ScrollBarVisibility.Auto,
            HorizontalScrollBarVisibility = ScrollBarVisibility.Disabled,
            Content = content is StackPanel ? WrapPad(content) : content,
        },
    };

    private static Border WrapPad(UIElement content) => new()
    {
        Padding = new Thickness(F.ContentPad),
        Child = content,
    };

    // ── General ──────────────────────────────────────────────────────────────
    private static UIElement BuildGeneral()
    {
        var panel = new StackPanel();

        panel.Children.Add(F.SectionHeader("General"));
        var autoStart = F.Check("Launch at Login", StartupRegistration.IsEnabled());
        autoStart.Checked += (_, _) => SetAutoStart(true);
        autoStart.Unchecked += (_, _) => SetAutoStart(false);
        autoStart.Margin = new Thickness(0, 0, 0, F.SectionGap);
        panel.Children.Add(autoStart);

        panel.Children.Add(F.SectionHeader("Updates"));
        var asmVer = System.Reflection.Assembly.GetExecutingAssembly().GetName().Version?.ToString() ?? "?";
        var displayVer = asmVer.EndsWith(".0") ? asmVer[..^2] : asmVer;
        panel.Children.Add(new WpfTextBlock
        {
            Text = $"Version: {displayVer}",
            Foreground = Theme.WpfBrush(Theme.TextMuted),
            Margin = new Thickness(0, 0, 0, F.FieldGap),
        });

        var updateLink = new HyperlinkButton { Visibility = Visibility.Collapsed, Margin = new Thickness(0, 0, 0, F.FieldGap) };
        updateLink.Click += (_, _) =>
        {
            var u = App.UpdateService?.AvailableUpdate;
            if (u != null) App.UpdateService!.StartInstall(u);
        };
        panel.Children.Add(updateLink);

        void RefreshUpdateLink()
        {
            var u = App.UpdateService?.AvailableUpdate;
            if (u != null)
            {
                var staged = App.UpdateService?.UpdateStaged ?? false;
                updateLink.Content = staged
                    ? $"Update {u.Version} downloaded — click to restart and install"
                    : $"Update {u.Version} available — click to download and install";
                updateLink.Visibility = Visibility.Visible;
            }
            else updateLink.Visibility = Visibility.Collapsed;
        }
        RefreshUpdateLink();

        var updateBtn = F.SecondaryButton("Check for Updates", () => { });
        updateBtn.Click += async (_, _) =>
        {
            updateBtn.IsEnabled = false;
            updateBtn.Content = "Checking...";
            if (App.UpdateService != null) await App.UpdateService.CheckNowAsync();
            RefreshUpdateLink();
            updateBtn.Content = "Check for Updates";
            updateBtn.IsEnabled = true;
        };
        panel.Children.Add(updateBtn);

        // Background re-poll so a just-published update surfaces here.
        if (App.UpdateService != null)
        {
            _ = App.UpdateService.RefreshAvailableUpdateAsync().ContinueWith(_ =>
            {
                try { updateLink.Dispatcher.BeginInvoke(new Action(RefreshUpdateLink)); }
                catch { /* window closed */ }
            });
        }

        return panel;
    }

    private static void SetAutoStart(bool on)
    {
        StartupRegistration.SetEnabled(on);
        App.Settings.AutoStart = on;
        App.Settings.Save();
        KeepAliveAgent.Reconcile(desiredRunAtLogon: on);
    }

    // ── Rewrite ──────────────────────────────────────────────────────────────
    private static UIElement BuildRewrite()
    {
        var panel = new StackPanel();
        var allTones = Enum.GetValues<Tone>();

        panel.Children.Add(F.SectionHeader("Mode"));
        panel.Children.Add(F.FieldLabel("Interaction Mode"));
        var modeCombo = F.Dropdown();
        modeCombo.Items.Add(AppMode.Advanced.DisplayName());
        modeCombo.Items.Add(AppMode.OneClick.DisplayName());
        modeCombo.SelectedIndex = App.Settings.AppMode == AppMode.OneClick ? 1 : 0;
        modeCombo.Margin = new Thickness(0, 0, 0, F.SectionGap);
        panel.Children.Add(modeCombo);

        // ── Simple block: One-Click tone ──
        var simpleBlock = new StackPanel();
        simpleBlock.Children.Add(F.FieldLabel($"{AppMode.OneClick.DisplayName()} Tone"));
        var oneClickCombo = F.Dropdown();
        foreach (var t in allTones) oneClickCombo.Items.Add(t.DisplayName());
        oneClickCombo.SelectedIndex = IndexOfApiValue(allTones, App.Settings.OneClickTone);
        oneClickCombo.SelectionChanged += (_, _) =>
        {
            if (oneClickCombo.SelectedIndex is >= 0)
            {
                App.Settings.OneClickTone = allTones[oneClickCombo.SelectedIndex].ApiValue();
                App.Settings.Save();
            }
        };
        oneClickCombo.Margin = new Thickness(0, 0, 0, F.SectionGap);
        simpleBlock.Children.Add(oneClickCombo);
        panel.Children.Add(simpleBlock);

        // ── Advanced block: Panel tones + Default tone ──
        var advancedBlock = new StackPanel();
        advancedBlock.Children.Add(F.SectionHeader("Panel Tones"));
        foreach (var tone in allTones)
        {
            var cb = F.Check($"{tone.Icon()}  {tone.DisplayName()}",
                App.Settings.EnabledTones.Contains(tone.ApiValue()));
            cb.Margin = new Thickness(0, 0, 0, F.LabelGap);
            var apiVal = tone.ApiValue();
            cb.Checked += (_, _) => ToggleTone(apiVal, true);
            cb.Unchecked += (_, _) => ToggleTone(apiVal, false);
            advancedBlock.Children.Add(cb);
        }
        advancedBlock.Children.Add(new WpfTextBlock { Height = F.LabelGap }); // small gap
        advancedBlock.Children.Add(F.FieldLabel("Default Tone (auto-run)"));
        var defaultCombo = F.Dropdown();
        foreach (var t in allTones) defaultCombo.Items.Add(t.DisplayName());
        defaultCombo.SelectedIndex = IndexOfApiValue(allTones, App.Settings.DefaultTone);
        defaultCombo.SelectionChanged += (_, _) =>
        {
            if (defaultCombo.SelectedIndex is >= 0)
            {
                App.Settings.DefaultTone = allTones[defaultCombo.SelectedIndex].ApiValue();
                App.Settings.Save();
            }
        };
        defaultCombo.Margin = new Thickness(0, 0, 0, F.SectionGap);
        advancedBlock.Children.Add(defaultCombo);
        panel.Children.Add(advancedBlock);

        // ── Translation (always visible) ──
        panel.Children.Add(F.SectionHeader("Translation"));
        panel.Children.Add(F.FieldLabel("Target Language"));
        var langBox = F.Dropdown(editable: true);
        foreach (var l in CommonLanguages) langBox.Items.Add(l);
        langBox.Text = App.Settings.TranslateLanguage ?? "";
        langBox.LostFocus += (_, _) => SaveLanguage(langBox.Text);
        langBox.KeyUp += (_, _) => SaveLanguage(langBox.Text);
        panel.Children.Add(langBox);

        void UpdateModeVisibility()
        {
            bool isSimple = modeCombo.SelectedIndex == 1;
            simpleBlock.Visibility = isSimple ? Visibility.Visible : Visibility.Collapsed;
            advancedBlock.Visibility = isSimple ? Visibility.Collapsed : Visibility.Visible;
        }
        UpdateModeVisibility();
        modeCombo.SelectionChanged += (_, _) =>
        {
            App.Settings.AppMode = modeCombo.SelectedIndex == 1 ? AppMode.OneClick : AppMode.Advanced;
            App.Settings.Save();
            UpdateModeVisibility();
        };

        return panel;
    }

    private static readonly string[] CommonLanguages =
    {
        "English", "Vietnamese", "Spanish", "French", "German", "Italian",
        "Portuguese", "Dutch", "Russian", "Japanese", "Korean",
        "Chinese (Simplified)", "Chinese (Traditional)", "Arabic", "Hindi",
        "Thai", "Indonesian", "Turkish", "Polish",
    };

    private static int IndexOfApiValue(Tone[] tones, string? apiValue)
    {
        for (int i = 0; i < tones.Length; i++)
            if (tones[i].ApiValue() == apiValue) return i;
        return 0;
    }

    private static void ToggleTone(string apiVal, bool on)
    {
        if (on) { if (!App.Settings.EnabledTones.Contains(apiVal)) App.Settings.EnabledTones.Add(apiVal); }
        else App.Settings.EnabledTones.Remove(apiVal);
        App.Settings.Save();
    }

    private static void SaveLanguage(string text)
    {
        var v = text.Trim();
        if (App.Settings.TranslateLanguage == v) return;
        App.Settings.TranslateLanguage = v;
        App.Settings.Save();
    }

    // ── Trigger ──────────────────────────────────────────────────────────────
    private static readonly TriggerMode[] TriggerModes =
        { TriggerMode.Pencil, TriggerMode.Hotkey };

    private static UIElement BuildTrigger()
    {
        var panel = new StackPanel();
        panel.Children.Add(F.SectionHeader("Rewrite Trigger"));
        panel.Children.Add(F.FieldLabel("Trigger"));
        var modeCombo = F.Dropdown();
        foreach (var m in TriggerModes) modeCombo.Items.Add(m.DisplayName());
        modeCombo.SelectedIndex = Math.Max(0, Array.IndexOf(TriggerModes, App.Settings.TriggerMode));
        modeCombo.Margin = new Thickness(0, 0, 0, F.SectionGap);
        modeCombo.SelectionChanged += (_, _) =>
        {
            if (modeCombo.SelectedIndex is >= 0)
            {
                App.Settings.TriggerMode = TriggerModes[modeCombo.SelectedIndex];
                App.Settings.Save();
                App.ApplyTriggerMode();
            }
        };
        panel.Children.Add(modeCombo);

        // The hotkey is used by the Hotkey and Both modes.
        panel.Children.Add(F.SectionHeader("Hotkey"));
        panel.Children.Add(F.FieldLabel("Current Hotkey"));
        panel.Children.Add(new WpfTextBlock
        {
            Text = FormatHotkey(App.Settings.HotkeyModifiers, App.Settings.HotkeyKey),
            Foreground = Theme.WpfBrush(Theme.BrandBlue),
            FontWeight = FontWeights.Bold,
            Margin = new Thickness(0, 0, 0, F.SectionGap),
        });
        panel.Children.Add(F.Body(
            "Pencil: highlight text by dragging to show the rewrite button.\n" +
            "Hotkey: select text, then press the hotkey.\n" +
            "Both: the pencil and the hotkey both work.\n" +
            "Hotkey editing is not yet available in this build."));
        return panel;
    }

    private static string FormatHotkey(int mods, int key)
    {
        var parts = new List<string>();
        if ((mods & HotkeyService.MOD_CONTROL) != 0) parts.Add("Ctrl");
        if ((mods & HotkeyService.MOD_SHIFT) != 0) parts.Add("Shift");
        if ((mods & HotkeyService.MOD_ALT) != 0) parts.Add("Alt");
        if ((mods & HotkeyService.MOD_WIN) != 0) parts.Add("Win");
        parts.Add(key is >= 0x41 and <= 0x5A ? ((char)key).ToString() : $"Key0x{key:X2}");
        return string.Join(" + ", parts);
    }

    // ── Account ──────────────────────────────────────────────────────────────
    private static UIElement BuildAccount()
    {
        // A host whose content is rebuilt on sign in / sign out, so the tab
        // flips between Sign In and Signed In without reopening the window.
        var host = new Border { Padding = new Thickness(F.ContentPad) };
        PopulateAccount(host);
        return host;
    }

    private static void PopulateAccount(Border host)
    {
        var panel = new StackPanel();
        host.Child = panel;

        var signedIn = !string.IsNullOrEmpty(App.Auth.AccessToken);
        if (signedIn)
        {
            panel.Children.Add(F.SectionHeader("Signed In"));
            var email = App.Auth.CurrentEmail;
            panel.Children.Add(new WpfTextBlock
            {
                Text = string.IsNullOrEmpty(email) ? "Signed in" : $"Signed in as {email}",
                Foreground = Theme.WpfBrush(Theme.SuccessGreen),
                Margin = new Thickness(0, 0, 0, F.SectionGap),
            });
            panel.Children.Add(F.SecondaryButton("Sign Out", () =>
            {
                App.Auth.ClearTokens();
                App.Api.ClearToken();
                PopulateAccount(host);
            }));
            return;
        }

        panel.Children.Add(F.SectionHeader("Sign In"));
        panel.Children.Add(F.FieldLabel("Email"));
        var emailBox = F.TextField();
        panel.Children.Add(emailBox);
        panel.Children.Add(F.FieldLabel("Password"));
        var passBox = F.PasswordField();
        panel.Children.Add(passBox);

        var status = new InfoBar { IsClosable = false, IsOpen = false, Margin = new Thickness(0, 0, 0, F.FieldGap) };
        panel.Children.Add(status);
        void SetError(string msg)
        {
            if (string.IsNullOrEmpty(msg)) { status.IsOpen = false; return; }
            status.Severity = InfoBarSeverity.Error;
            status.Message = msg;
            status.IsOpen = true;
        }

        var signInBtn = F.PrimaryButton("Sign In", () => { });
        signInBtn.Click += async (_, _) =>
        {
            SetError("");
            var email = emailBox.Text.Trim();
            var password = passBox.Password;
            if (string.IsNullOrEmpty(email)) { SetError("Please enter your email."); return; }
            if (!IsValidEmail(email)) { SetError("Please enter a valid email address."); return; }
            if (string.IsNullOrEmpty(password)) { SetError("Please enter your password."); return; }

            signInBtn.IsEnabled = false;
            try
            {
                App.Api.SetBaseUrl(App.Settings.BackendUrl ?? "");
                var result = await App.Api.LoginAsync(email, password);
                if (!string.IsNullOrEmpty(result.AccessToken))
                {
                    App.Auth.SaveTokens(result.AccessToken, result.RefreshToken, result.User?.Email);
                    App.Api.SetToken(result.AccessToken);
                    PopulateAccount(host);
                }
                else SetError("Login failed.");
            }
            catch (ApiException apiEx)
            {
                SetError(apiEx.ServerMessage ?? apiEx.Message);
                DRLogger.Error($"Login API error: {apiEx}", DRLogger.Category.AUTH);
            }
            catch (Exception ex)
            {
                SetError("Something went wrong. Please try again.");
                DRLogger.Error($"Login error: {ex}", DRLogger.Category.AUTH);
            }
            finally { try { signInBtn.IsEnabled = true; } catch { } }
        };
        signInBtn.Margin = new Thickness(0, 0, 0, F.FieldGap);
        panel.Children.Add(signInBtn);

        var googleBtn = F.SecondaryButton("Continue with Google", () => { });
        googleBtn.Click += async (_, _) =>
        {
            SetError("");
            googleBtn.IsEnabled = false;
            signInBtn.IsEnabled = false;
            try
            {
                App.Api.SetBaseUrl(App.Settings.BackendUrl ?? "");
                var idToken = await GoogleOAuth.AuthenticateAsync();
                var result = await App.Api.SocialLoginAsync("google", idToken);
                if (!string.IsNullOrEmpty(result.AccessToken))
                {
                    App.Auth.SaveTokens(result.AccessToken, result.RefreshToken, result.User?.Email);
                    App.Api.SetToken(result.AccessToken);
                    PopulateAccount(host);
                }
                else SetError("Google sign-in failed.");
            }
            catch (ApiException apiEx)
            {
                SetError(apiEx.ServerMessage ?? apiEx.Message);
                DRLogger.Error($"Google sign-in API error: {apiEx}", DRLogger.Category.AUTH);
            }
            catch (Exception ex)
            {
                SetError(ex.Message);
                DRLogger.Error($"Google sign-in error: {ex}", DRLogger.Category.AUTH);
            }
            finally { try { googleBtn.IsEnabled = true; signInBtn.IsEnabled = true; } catch { } }
        };
        panel.Children.Add(googleBtn);
        panel.Children.Add(new WpfTextBlock
        {
            Text = "New to DraftRight? Use \"Continue with Google\" to create your account in seconds.",
            FontSize = 12,
            Foreground = Theme.WpfBrush(Theme.TextMuted),
            TextWrapping = TextWrapping.Wrap,
            Margin = new Thickness(0, F.LabelGap, 0, 0),
        });
    }

    private static bool IsValidEmail(string s)
    {
        if (string.IsNullOrWhiteSpace(s)) return false;
        var at = s.IndexOf('@');
        if (at <= 0 || at == s.Length - 1) return false;
        return s.IndexOf('.', at) > at;
    }

    // ── Advanced ─────────────────────────────────────────────────────────────
    private static UIElement BuildAdvanced()
    {
        var panel = new StackPanel();

        panel.Children.Add(F.SectionHeader("Logs"));
        var logging = F.Check("Enable Logging", App.Settings.LoggingEnabled);
        logging.Checked += (_, _) => SetLogging(true);
        logging.Unchecked += (_, _) => SetLogging(false);
        logging.Margin = new Thickness(0, 0, 0, F.FieldGap);
        panel.Children.Add(logging);

        panel.Children.Add(F.FieldLabel("Log file location:"));
        var logPath = DRLogger.LogFilePath;
        var pathBox = F.TextField(logPath);
        pathBox.IsReadOnly = true;
        panel.Children.Add(pathBox);

        var logButtons = new StackPanel { Orientation = System.Windows.Controls.Orientation.Horizontal, Margin = new Thickness(0, 0, 0, F.SectionGap) };
        var openBtn = F.SecondaryButton("Open", () => OpenLog(logPath));
        var clearBtn = F.SecondaryButton("Clear", () => ClearLog(logPath));
        clearBtn.Margin = new Thickness(F.ButtonGap, 0, 0, 0);
        logButtons.Children.Add(openBtn);
        logButtons.Children.Add(clearBtn);
        panel.Children.Add(logButtons);

        panel.Children.Add(F.SectionHeader("Feedback"));
        panel.Children.Add(new WpfTextBlock
        {
            Text = "Hit a bug? Send us a description (and a screenshot if you have one) and we'll take a look.",
            Foreground = Theme.WpfBrush(Theme.TextMuted),
            TextWrapping = TextWrapping.Wrap,
            Margin = new Thickness(0, 0, 0, F.FieldGap),
        });
        var feedbackButtons = new StackPanel { Orientation = System.Windows.Controls.Orientation.Horizontal };
        var bugBtn = F.SecondaryButton("Report a Bug…", () => ReportBugDialog.Show());
        var featureBtn = F.SecondaryButton("Suggest a feature…", () => SuggestFeatureDialog.Show());
        featureBtn.Margin = new Thickness(F.ButtonGap, 0, 0, 0);
        feedbackButtons.Children.Add(bugBtn);
        feedbackButtons.Children.Add(featureBtn);
        panel.Children.Add(feedbackButtons);

        return panel;
    }

    private static void SetLogging(bool on)
    {
        App.Settings.LoggingEnabled = on;
        App.Settings.Save();
        DRLogger.IsEnabled = on;
    }

    private static void OpenLog(string path)
    {
        if (System.IO.File.Exists(path))
            System.Diagnostics.Process.Start(new System.Diagnostics.ProcessStartInfo(path) { UseShellExecute = true });
        else
            System.Windows.MessageBox.Show("Log file does not exist yet.", "DraftRight",
                System.Windows.MessageBoxButton.OK, System.Windows.MessageBoxImage.Information);
    }

    private static void ClearLog(string path)
    {
        try { System.IO.File.WriteAllText(path, ""); }
        catch (Exception ex)
        {
            System.Windows.MessageBox.Show($"Could not clear log: {ex.Message}", "DraftRight",
                System.Windows.MessageBoxButton.OK, System.Windows.MessageBoxImage.Warning);
        }
    }
}
