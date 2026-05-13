using System;
using System.Drawing;
using System.IO;
using System.Threading;
using DraftRightWindows.Services;
using WinForms = System.Windows.Forms;

namespace DraftRightWindows.Views;

/// <summary>
/// "Suggest a feature" dialog. Mirrors <see cref="ReportBugDialog"/> for
/// STA-thread construction, dark theme, control layout, and async submit
/// pattern. Posts to <see cref="FeedbackService"/>.
/// </summary>
internal static class SuggestFeatureDialog
{
    // Theme — matches ReportBugDialog / SettingsFormBuilder
    private static readonly Color BgDark = Color.FromArgb(15, 23, 42);
    private static readonly Color CardBg = Color.FromArgb(30, 41, 59);
    private static readonly Color BrandBlue = Color.FromArgb(93, 135, 255);
    private static readonly Color TextPrimary = Color.FromArgb(226, 232, 240);
    private static readonly Color TextMuted = Color.FromArgb(148, 163, 184);
    private static readonly Color ErrorRed = Color.FromArgb(239, 68, 68);
    private static readonly Color SuccessGreen = Color.FromArgb(34, 197, 94);
    private static readonly Color BorderColor = Color.FromArgb(51, 65, 85);

    /// <summary>
    /// Tiny value type that lets a ComboBox display a human-readable label
    /// while we read the backend-facing value string on submit.
    /// </summary>
    private sealed record PlatformOpt(string Value, string Label)
    {
        public override string ToString() => Label;
    }

    /// <summary>
    /// Opens the dialog on its own STA thread so it works regardless of which
    /// thread the caller (tray menu, settings, hotkey…) is on.
    /// </summary>
    public static void Show(IntPtr ownerHwnd = default)
    {
        var thread = new Thread(() => RunDialog(ownerHwnd));
        thread.SetApartmentState(ApartmentState.STA);
        thread.IsBackground = true;
        thread.Start();
    }

    private static void RunDialog(IntPtr ownerHwnd)
    {
        WinForms.Application.EnableVisualStyles();

        var form = new WinForms.Form
        {
            Text = "Suggest a feature",
            Width = 560,
            Height = 640,
            StartPosition = WinForms.FormStartPosition.CenterScreen,
            BackColor = BgDark,
            ForeColor = TextPrimary,
            FormBorderStyle = WinForms.FormBorderStyle.FixedSingle,
            MaximizeBox = false,
            MinimizeBox = false,
            ShowInTaskbar = true,
        };

        var exePath = Environment.ProcessPath;
        if (exePath != null)
        {
            var icoPath = Path.Combine(Path.GetDirectoryName(exePath)!, "Assets", "DraftRight.ico");
            if (File.Exists(icoPath))
            {
                try { form.Icon = new Icon(icoPath); } catch { /* best-effort icon */ }
            }
        }

        // ── Layout ───────────────────────────────────────────
        int y = 16;

        var titleHeading = new WinForms.Label
        {
            Text = "Suggest a feature",
            Font = new Font("Segoe UI", 14, FontStyle.Bold),
            ForeColor = TextPrimary,
            Location = new Point(20, y),
            AutoSize = true,
        };
        form.Controls.Add(titleHeading);
        y += 32;

        var subtitle = new WinForms.Label
        {
            Text = "Got an idea? We'd love to hear it.",
            Font = new Font("Segoe UI", 9),
            ForeColor = TextMuted,
            Location = new Point(20, y),
            AutoSize = true,
        };
        form.Controls.Add(subtitle);
        y += 28;

        // Title field
        form.Controls.Add(new WinForms.Label
        {
            Text = "Title",
            Font = new Font("Segoe UI", 9),
            ForeColor = TextMuted,
            Location = new Point(20, y),
            AutoSize = true,
        });
        y += 18;

        var titleBox = new WinForms.TextBox
        {
            MaxLength = 80,
            Multiline = false,
            Location = new Point(20, y),
            Size = new Size(500, 30),
            BackColor = CardBg,
            ForeColor = TextPrimary,
            BorderStyle = WinForms.BorderStyle.FixedSingle,
            Font = new Font("Segoe UI", 10),
        };
        form.Controls.Add(titleBox);
        y += 44;

        // Target platform
        form.Controls.Add(new WinForms.Label
        {
            Text = "Target platform",
            Font = new Font("Segoe UI", 9),
            ForeColor = TextMuted,
            Location = new Point(20, y),
            AutoSize = true,
        });
        y += 18;

        var platformCombo = new WinForms.ComboBox
        {
            DropDownStyle = WinForms.ComboBoxStyle.DropDownList,
            Location = new Point(20, y),
            Size = new Size(240, 30),
            BackColor = CardBg,
            ForeColor = TextPrimary,
            FlatStyle = WinForms.FlatStyle.Flat,
            Font = new Font("Segoe UI", 10),
        };
        platformCombo.Items.AddRange(new object[]
        {
            new PlatformOpt("playground", "Playground (web)"),
            new PlatformOpt("mobile",     "Mobile (iOS / Android)"),
            new PlatformOpt("windows",    "Windows"),
            new PlatformOpt("mac",        "macOS"),
            new PlatformOpt("linux",      "Linux"),
        });
        platformCombo.SelectedIndex = 2; // Default: Windows
        form.Controls.Add(platformCombo);
        y += 44;

        // Details field
        form.Controls.Add(new WinForms.Label
        {
            Text = "Describe the feature",
            Font = new Font("Segoe UI", 9),
            ForeColor = TextMuted,
            Location = new Point(20, y),
            AutoSize = true,
        });
        y += 18;

        var detailsBox = new WinForms.TextBox
        {
            MaxLength = 2000,
            Multiline = true,
            AcceptsReturn = true,
            ScrollBars = WinForms.ScrollBars.Vertical,
            Location = new Point(20, y),
            Size = new Size(500, 120),
            BackColor = CardBg,
            ForeColor = TextPrimary,
            BorderStyle = WinForms.BorderStyle.FixedSingle,
            Font = new Font("Segoe UI", 10),
        };
        form.Controls.Add(detailsBox);
        y += 132;

        // Email — only shown when not signed in (mirrors ReportBugDialog)
        bool isSignedIn = false;
        string? signedInEmail = null;
        try
        {
            isSignedIn = App.Auth?.IsLoggedIn ?? false;
            signedInEmail = App.Auth?.CurrentEmail;
        }
        catch
        {
            // App.Auth may be null in test/standalone scenarios.
        }

        WinForms.TextBox? emailBox = null;
        if (!isSignedIn)
        {
            form.Controls.Add(new WinForms.Label
            {
                Text = "Email (optional — so we can follow up)",
                Font = new Font("Segoe UI", 9),
                ForeColor = TextMuted,
                Location = new Point(20, y),
                AutoSize = true,
            });
            y += 18;

            emailBox = new WinForms.TextBox
            {
                Location = new Point(20, y),
                Size = new Size(500, 30),
                BackColor = CardBg,
                ForeColor = TextPrimary,
                BorderStyle = WinForms.BorderStyle.FixedSingle,
                Font = new Font("Segoe UI", 10),
            };
            form.Controls.Add(emailBox);
            y += 44;
        }
        else
        {
            form.Controls.Add(new WinForms.Label
            {
                Text = $"Submitting as {signedInEmail}",
                Font = new Font("Segoe UI", 9, FontStyle.Italic),
                ForeColor = SuccessGreen,
                Location = new Point(20, y),
                AutoSize = true,
            });
            y += 28;
        }

        // "See all requests" link
        var seeAllLink = new WinForms.LinkLabel
        {
            Text = "See all requests →",
            Font = new Font("Segoe UI", 9),
            LinkColor = BrandBlue,
            Location = new Point(20, y),
            AutoSize = true,
        };
        seeAllLink.LinkClicked += (_, _) =>
        {
            try
            {
                System.Diagnostics.Process.Start(new System.Diagnostics.ProcessStartInfo(
                    "https://draftright.info/feedback")
                { UseShellExecute = true });
            }
            catch { /* best-effort */ }
        };
        form.Controls.Add(seeAllLink);
        y += 28;

        // Status / error label
        var statusLabel = new WinForms.Label
        {
            Text = "",
            Font = new Font("Segoe UI", 9),
            ForeColor = ErrorRed,
            Location = new Point(20, y),
            Size = new Size(500, 36),
            AutoEllipsis = true,
            Visible = false,
        };
        form.Controls.Add(statusLabel);
        y += 44;

        // Buttons — bottom-right aligned
        var cancelBtn = new WinForms.Button
        {
            Text = "Cancel",
            Location = new Point(360, y),
            Size = new Size(80, 32),
            BackColor = CardBg,
            ForeColor = TextPrimary,
            FlatStyle = WinForms.FlatStyle.Flat,
            Font = new Font("Segoe UI", 9.5f),
        };
        cancelBtn.FlatAppearance.BorderColor = BorderColor;
        form.Controls.Add(cancelBtn);

        var submitBtn = new WinForms.Button
        {
            Text = "Submit",
            Location = new Point(448, y),
            Size = new Size(72, 32),
            BackColor = BrandBlue,
            ForeColor = Color.White,
            FlatStyle = WinForms.FlatStyle.Flat,
            Font = new Font("Segoe UI", 9.5f, FontStyle.Bold),
            Enabled = false, // starts disabled until title + details are filled
        };
        submitBtn.FlatAppearance.BorderSize = 0;
        form.Controls.Add(submitBtn);

        // ── Helpers ─────────────────────────────────────────

        void SetStatus(string text, Color color)
        {
            statusLabel.Text = text;
            statusLabel.ForeColor = color;
            statusLabel.Visible = !string.IsNullOrEmpty(text);
        }

        void RefreshSubmitEnabled()
        {
            submitBtn.Enabled =
                titleBox.Text.Trim().Length > 0 &&
                detailsBox.Text.Trim().Length > 0;
        }

        // ── Wiring ──────────────────────────────────────────

        titleBox.TextChanged += (_, _) => RefreshSubmitEnabled();
        detailsBox.TextChanged += (_, _) => RefreshSubmitEnabled();

        cancelBtn.Click += (_, _) => form.Close();

        // Submit
        submitBtn.Click += async (_, _) =>
        {
            var titleText = titleBox.Text?.Trim() ?? "";
            var detailsText = detailsBox.Text?.Trim() ?? "";

            if (titleText.Length == 0)
            {
                SetStatus("Please enter a title.", ErrorRed);
                titleBox.Focus();
                return;
            }
            if (detailsText.Length == 0)
            {
                SetStatus("Please describe the feature.", ErrorRed);
                detailsBox.Focus();
                return;
            }

            var platform = ((PlatformOpt)platformCombo.SelectedItem!).Value;
            var email = emailBox?.Text?.Trim();
            string? authToken = null;
            try { authToken = App.Auth?.AccessToken; } catch { authToken = null; }

            // Lock UI
            submitBtn.Enabled = false;
            cancelBtn.Enabled = false;
            titleBox.ReadOnly = true;
            detailsBox.ReadOnly = true;
            if (emailBox != null) emailBox.ReadOnly = true;
            platformCombo.Enabled = false;
            SetStatus("Submitting…", TextMuted);

            try
            {
                var result = await FeedbackService.SubmitAsync(
                    title: titleText,
                    targetPlatform: platform,
                    description: detailsText,
                    userEmail: !string.IsNullOrWhiteSpace(email) ? email : null,
                    authToken: authToken);

                if (result.Success)
                {
                    DRLogger.Log($"Feature request submitted (id={result.Id ?? "?"})", DRLogger.Category.APP);
                    SetStatus("Thanks! Your suggestion was submitted.", SuccessGreen);
                    // Brief flash of success, then close — mirrors ReportBugDialog.
                    var t = new WinForms.Timer { Interval = 900 };
                    t.Tick += (_, _) =>
                    {
                        t.Stop();
                        t.Dispose();
                        form.Close();
                    };
                    t.Start();
                }
                else
                {
                    SetStatus(result.ErrorMessage ?? "Submit failed.", ErrorRed);
                    submitBtn.Enabled = true;
                    cancelBtn.Enabled = true;
                    titleBox.ReadOnly = false;
                    detailsBox.ReadOnly = false;
                    if (emailBox != null) emailBox.ReadOnly = false;
                    platformCombo.Enabled = true;
                }
            }
            catch (Exception ex)
            {
                DRLogger.Log($"Feature request submit threw: {ex}", DRLogger.Category.APP);
                SetStatus(ex.Message, ErrorRed);
                submitBtn.Enabled = true;
                cancelBtn.Enabled = true;
                titleBox.ReadOnly = false;
                detailsBox.ReadOnly = false;
                if (emailBox != null) emailBox.ReadOnly = false;
                platformCombo.Enabled = true;
            }
        };

        form.AcceptButton = submitBtn;
        form.CancelButton = cancelBtn;

        WinForms.Application.Run(form);
    }
}
