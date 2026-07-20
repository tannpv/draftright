using System;
using System.ComponentModel;
using System.Drawing;
using System.IO;
using System.Linq;
using DraftRightWindows.Models;
using DraftRightWindows.Services;
using DraftRightWindows.ViewModels;
using WinForms = System.Windows.Forms;

namespace DraftRightWindows.Views;

/// <summary>
/// WinForms implementation of the rewrite panel.
/// Replaces the WinUI version, which crashes on unpackaged builds because
/// XAML theme resources can't be resolved without a properly generated PRI file
/// (the standalone .NET SDK doesn't ship the AppX MSBuild tooling that would
/// build that PRI).
///
/// Public surface mirrors the old WinUI RewritePanel:
///   - ViewModel: RewritePanelViewModel
///   - SetInputText(text)
///   - Show() / Close()
/// so App.cs can use it the same way.
/// </summary>
public sealed class RewritePanelForm : WinForms.Form
{
    public RewritePanelViewModel ViewModel { get; }

    // ── Theme constants (match the dark theme used elsewhere) ─────────
    private static readonly Color BgDark      = Color.FromArgb(15, 23, 42);
    private static readonly Color CardBg      = Color.FromArgb(30, 41, 59);
    private static readonly Color ResultBg    = Color.FromArgb(15, 41, 34);
    private static readonly Color BrandBlue   = Color.FromArgb(93, 135, 255);
    private static readonly Color TextPrimary = Color.FromArgb(226, 232, 240);
    private static readonly Color TextMuted   = Color.FromArgb(148, 163, 184);
    private static readonly Color BorderColor = Color.FromArgb(51, 65, 85);
    private static readonly Color ErrorRed    = Color.FromArgb(239, 68, 68);
    private static readonly Color SuccessGreen = Color.FromArgb(34, 197, 94);

    // ── Tone button layout: 6 visible tones in a 3×2 grid ─────────────
    private static readonly (Tone Tone, string Icon, string Label)[] ToneButtonsLayout =
    {
        (Tone.Simple,    "✎",      "Simple"),
        (Tone.Natural,   "\U0001F4AC", "Natural"),
        (Tone.Polished,  "✨",      "Polished"),
        (Tone.Concise,   "⊖",      "Concise"),
        (Tone.Technical, "\U0001F527", "Technical"),
        (Tone.Translate, "\U0001F310", "Translate"),
    };

    // Control refs we need to update from ViewModel changes
    private readonly WinForms.TextBox _inputBox;
    private readonly WinForms.TextBox _outputBox;
    private readonly WinForms.Label _usageLabel;
    private readonly WinForms.Label _errorLabel;
    private readonly WinForms.Button _copyErrorBtn;
    private readonly WinForms.Button _replaceBtn;
    private readonly WinForms.Button _copyBtn;
    private readonly WinForms.Label _loadingLabel;
    private readonly System.Collections.Generic.Dictionary<Tone, WinForms.Button> _toneButtons = new();

    public RewritePanelForm()
    {
        ViewModel = new RewritePanelViewModel();

        // ── Form chrome ──
        Text = "DraftRight — Rewrite";
        Width = 640;
        Height = 600;
        StartPosition = WinForms.FormStartPosition.CenterScreen;
        BackColor = BgDark;
        ForeColor = TextPrimary;
        FormBorderStyle = WinForms.FormBorderStyle.Sizable;
        MinimumSize = new Size(480, 460);
        ShowInTaskbar = true;
        TopMost = true;
        Padding = new WinForms.Padding(16);

        SetIcon();

        // ── Layout: TableLayoutPanel with rows for each section ──
        var root = new WinForms.TableLayoutPanel
        {
            Dock = WinForms.DockStyle.Fill,
            ColumnCount = 1,
            RowCount = 6,
            BackColor = BgDark,
        };
        root.ColumnStyles.Add(new WinForms.ColumnStyle(WinForms.SizeType.Percent, 100));
        root.RowStyles.Add(new WinForms.RowStyle(WinForms.SizeType.AutoSize));         // input
        root.RowStyles.Add(new WinForms.RowStyle(WinForms.SizeType.AutoSize));         // tones
        root.RowStyles.Add(new WinForms.RowStyle(WinForms.SizeType.Percent, 100));     // output
        root.RowStyles.Add(new WinForms.RowStyle(WinForms.SizeType.AutoSize));         // usage
        root.RowStyles.Add(new WinForms.RowStyle(WinForms.SizeType.AutoSize));         // actions
        root.RowStyles.Add(new WinForms.RowStyle(WinForms.SizeType.AutoSize));         // error

        // Row 0: Input preview (multi-line read-only TextBox)
        _inputBox = new WinForms.TextBox
        {
            Multiline = true,
            ReadOnly = true,
            ScrollBars = WinForms.ScrollBars.Vertical,
            BackColor = CardBg,
            ForeColor = TextMuted,
            BorderStyle = WinForms.BorderStyle.FixedSingle,
            Font = new Font("Segoe UI", 9.5f),
            Dock = WinForms.DockStyle.Fill,
            Height = 70,
            Margin = new WinForms.Padding(0, 0, 0, 12),
        };
        root.Controls.Add(_inputBox, 0, 0);

        // Row 1: Tone button grid
        root.Controls.Add(BuildToneGrid(), 0, 1);

        // Row 2: Output area + loading indicator overlay
        root.Controls.Add(BuildOutputArea(out _outputBox, out _loadingLabel), 0, 2);

        // Row 3: Usage info
        _usageLabel = new WinForms.Label
        {
            Text = "",
            ForeColor = TextMuted,
            Font = new Font("Segoe UI", 9),
            AutoSize = true,
            Margin = new WinForms.Padding(2, 4, 0, 6),
        };
        root.Controls.Add(_usageLabel, 0, 3);

        // Row 4: Action buttons (Replace | Copy | Close)
        root.Controls.Add(BuildActionButtons(out _replaceBtn, out _copyBtn), 0, 4);

        // Row 5: Error message + copy button (hidden until there's an error)
        root.Controls.Add(BuildErrorRow(out _errorLabel, out _copyErrorBtn), 0, 5);

        Controls.Add(root);

        // ── Wire ViewModel ↔ View ──
        ViewModel.PropertyChanged += OnViewModelPropertyChanged;
        ViewModel.CloseRequested += (_, _) => SafeClose();
        FormClosed += (_, _) => ViewModel.PropertyChanged -= OnViewModelPropertyChanged;

        // Sync initial state
        SyncReplaceCopyEnabled();
        SyncLoadingState();
    }

    /// <summary>Sets the captured input text. Call before Show().</summary>
    public void SetInputText(string text) => ViewModel.InputText = text ?? string.Empty;

    /// <summary>
    /// Force the form to the foreground. Windows blocks SetForegroundWindow when the
    /// caller doesn't own the active foreground; the workaround is to attach our
    /// thread's input queue to the current foreground thread, set foreground, detach.
    /// We do this on Shown because at that point the form is fully created and the
    /// hotkey-press timestamp still grants our process foreground privileges.
    /// </summary>
    protected override void OnShown(EventArgs e)
    {
        base.OnShown(e);
        try
        {
            // Toggle TopMost to nudge the Z-order without leaving the form pinned above
            // every other window forever.
            TopMost = true;
            BringToFront();
            Activate();
            Focus();

            // Belt-and-suspenders: AttachThreadInput trick for stubborn cases.
            var foreHwnd = Helpers.Win32Interop.GetForegroundWindow();
            if (foreHwnd != IntPtr.Zero && foreHwnd != Handle)
            {
                uint foreThread = Helpers.Win32Interop.GetWindowThreadProcessId(foreHwnd, out _);
                uint thisThread = (uint)System.Threading.Thread.CurrentThread.ManagedThreadId;
                // Note: ManagedThreadId is not the same as native thread id. Use GetCurrentThreadId.
                uint nativeThis = GetCurrentThreadId();
                if (foreThread != 0 && foreThread != nativeThis)
                {
                    AttachThreadInput(nativeThis, foreThread, true);
                    Helpers.Win32Interop.SetForegroundWindow(Handle);
                    AttachThreadInput(nativeThis, foreThread, false);
                }
            }

            // Drop TopMost after a short delay so the panel doesn't stay above
            // dialogs the user might open later. Re-apply via a one-shot timer.
            var t = new WinForms.Timer { Interval = 250 };
            t.Tick += (_, _) => { TopMost = false; t.Stop(); t.Dispose(); };
            t.Start();
        }
        catch (Exception ex) { DRLogger.Warn($"Foreground-grab failed: {ex.Message}", DRLogger.Category.PANEL); }

        // Advanced mode "Default Tone (auto-run)": run the configured tone on
        // open so the user gets a rewrite without clicking. Null = manual pick.
        if (AutoRunTone is Tone autoTone)
        {
            DRLogger.Log($"Panel: auto-running default tone {autoTone}", DRLogger.Category.PANEL);
            RunTone(autoTone);
        }
    }

    [System.Runtime.InteropServices.DllImport("user32.dll")]
    private static extern bool AttachThreadInput(uint idAttach, uint idAttachTo, bool fAttach);

    [System.Runtime.InteropServices.DllImport("kernel32.dll")]
    private static extern uint GetCurrentThreadId();

    private void SetIcon()
    {
        // Embedded-resource load — survives single-file publish where the .ico
        // isn't next to the exe (#78). See Helpers/AppIcon.
        var ico = Helpers.AppIcon.Load();
        if (ico != null) Icon = ico;
    }

    private WinForms.TableLayoutPanel BuildToneGrid()
    {
        var grid = new WinForms.TableLayoutPanel
        {
            Dock = WinForms.DockStyle.Fill,
            ColumnCount = 3,
            RowCount = 2,
            BackColor = BgDark,
            Margin = new WinForms.Padding(0, 0, 0, 12),
            AutoSize = true,
            AutoSizeMode = WinForms.AutoSizeMode.GrowAndShrink,
        };
        for (int i = 0; i < 3; i++) grid.ColumnStyles.Add(new WinForms.ColumnStyle(WinForms.SizeType.Percent, 33.33f));
        for (int i = 0; i < 2; i++) grid.RowStyles.Add(new WinForms.RowStyle(WinForms.SizeType.AutoSize));

        for (int i = 0; i < ToneButtonsLayout.Length; i++)
        {
            var (tone, icon, label) = ToneButtonsLayout[i];
            int col = i % 3;
            int row = i / 3;

            var btn = new WinForms.Button
            {
                Text = $"{icon}  {label}",
                Tag = tone,
                Dock = WinForms.DockStyle.Fill,
                Height = 44,
                Margin = new WinForms.Padding(4),
                BackColor = CardBg,
                ForeColor = TextPrimary,
                FlatStyle = WinForms.FlatStyle.Flat,
                Font = new Font("Segoe UI", 10),
                TextAlign = ContentAlignment.MiddleCenter,
                Cursor = WinForms.Cursors.Hand,
            };
            btn.FlatAppearance.BorderColor = BorderColor;
            btn.FlatAppearance.BorderSize = 1;
            btn.Click += OnToneClick;
            _toneButtons[tone] = btn;
            grid.Controls.Add(btn, col, row);
        }

        return grid;
    }

    private WinForms.Panel BuildOutputArea(out WinForms.TextBox outputBox, out WinForms.Label loadingLabel)
    {
        var panel = new WinForms.Panel
        {
            Dock = WinForms.DockStyle.Fill,
            BackColor = BgDark,
            Margin = new WinForms.Padding(0, 0, 0, 8),
            MinimumSize = new Size(0, 200),
        };

        outputBox = new WinForms.TextBox
        {
            Multiline = true,
            ReadOnly = true,
            ScrollBars = WinForms.ScrollBars.Vertical,
            BackColor = ResultBg,
            ForeColor = TextPrimary,
            BorderStyle = WinForms.BorderStyle.FixedSingle,
            Font = new Font("Segoe UI", 10.5f),
            Dock = WinForms.DockStyle.Fill,
        };
        panel.Controls.Add(outputBox);

        // Loading overlay — a Label centered on top of the output box.
        loadingLabel = new WinForms.Label
        {
            Text = "Loading…",
            Font = new Font("Segoe UI", 11, FontStyle.Bold),
            ForeColor = BrandBlue,
            BackColor = ResultBg,
            TextAlign = ContentAlignment.MiddleCenter,
            Dock = WinForms.DockStyle.Fill,
            Visible = false,
        };
        panel.Controls.Add(loadingLabel);
        loadingLabel.BringToFront();

        return panel;
    }

    private WinForms.FlowLayoutPanel BuildActionButtons(out WinForms.Button replaceBtn, out WinForms.Button copyBtn)
    {
        var flow = new WinForms.FlowLayoutPanel
        {
            Dock = WinForms.DockStyle.Fill,
            FlowDirection = WinForms.FlowDirection.RightToLeft,
            BackColor = BgDark,
            AutoSize = true,
            AutoSizeMode = WinForms.AutoSizeMode.GrowAndShrink,
            Margin = new WinForms.Padding(0, 4, 0, 4),
        };

        var closeBtn = new WinForms.Button
        {
            Text = "Close",
            Size = new Size(96, 34),
            BackColor = CardBg,
            ForeColor = TextPrimary,
            FlatStyle = WinForms.FlatStyle.Flat,
            Font = new Font("Segoe UI", 10),
            Cursor = WinForms.Cursors.Hand,
            Margin = new WinForms.Padding(4),
        };
        closeBtn.FlatAppearance.BorderColor = BorderColor;
        closeBtn.Click += (_, _) => SafeClose();

        copyBtn = new WinForms.Button
        {
            Text = "Copy",
            Size = new Size(96, 34),
            BackColor = CardBg,
            ForeColor = TextPrimary,
            FlatStyle = WinForms.FlatStyle.Flat,
            Font = new Font("Segoe UI", 10),
            Cursor = WinForms.Cursors.Hand,
            Margin = new WinForms.Padding(4),
            Enabled = false,
        };
        copyBtn.FlatAppearance.BorderColor = BorderColor;
        var copyBtnRef = copyBtn;
        copyBtn.Click += (_, _) =>
        {
            if (string.IsNullOrEmpty(ViewModel.OutputText)) return;
            try { WinForms.Clipboard.SetText(ViewModel.OutputText); copyBtnRef.Text = "Copied!"; }
            catch { /* clipboard might be locked */ }
        };

        replaceBtn = new WinForms.Button
        {
            Text = "Replace",
            Size = new Size(110, 34),
            BackColor = BrandBlue,
            ForeColor = Color.White,
            FlatStyle = WinForms.FlatStyle.Flat,
            Font = new Font("Segoe UI", 10, FontStyle.Bold),
            Cursor = WinForms.Cursors.Hand,
            Margin = new WinForms.Padding(4),
            Enabled = false,
        };
        replaceBtn.FlatAppearance.BorderSize = 0;
        replaceBtn.Click += (_, _) =>
        {
            if (ViewModel.ReplaceCommand.CanExecute(null))
                ViewModel.ReplaceCommand.Execute(null);
        };

        // RTL flow: add in reverse so left-to-right order is Replace, Copy, Close
        flow.Controls.Add(closeBtn);
        flow.Controls.Add(copyBtn);
        flow.Controls.Add(replaceBtn);

        return flow;
    }

    private WinForms.FlowLayoutPanel BuildErrorRow(out WinForms.Label errorLabel, out WinForms.Button copyErrorBtn)
    {
        var flow = new WinForms.FlowLayoutPanel
        {
            Dock = WinForms.DockStyle.Fill,
            FlowDirection = WinForms.FlowDirection.LeftToRight,
            AutoSize = true,
            AutoSizeMode = WinForms.AutoSizeMode.GrowAndShrink,
            BackColor = BgDark,
            Margin = new WinForms.Padding(0, 6, 0, 0),
        };

        errorLabel = new WinForms.Label
        {
            Text = "",
            ForeColor = ErrorRed,
            Font = new Font("Segoe UI", 9),
            AutoSize = true,
            MaximumSize = new Size(540, 0),
            Margin = new WinForms.Padding(2, 6, 8, 0),
            Visible = false,
        };

        copyErrorBtn = new WinForms.Button
        {
            Text = "Copy",
            Size = new Size(70, 26),
            BackColor = CardBg,
            ForeColor = ErrorRed,
            FlatStyle = WinForms.FlatStyle.Flat,
            Font = new Font("Segoe UI", 9),
            Cursor = WinForms.Cursors.Hand,
            Margin = new WinForms.Padding(0, 2, 0, 0),
            Visible = false,
        };
        copyErrorBtn.FlatAppearance.BorderColor = ErrorRed;
        var errorLabelRef = errorLabel;
        copyErrorBtn.Click += (_, _) =>
        {
            if (string.IsNullOrEmpty(errorLabelRef.Text)) return;
            try { WinForms.Clipboard.SetText(errorLabelRef.Text); }
            catch { /* clipboard might be locked */ }
        };

        flow.Controls.Add(errorLabel);
        flow.Controls.Add(copyErrorBtn);
        return flow;
    }

    /// <summary>
    /// Tone to run automatically when the panel first opens (Advanced mode's
    /// "Default Tone (auto-run)" setting), so the user sees a rewrite without
    /// clicking a tone. Null = wait for a manual click (the prior behavior).
    /// Set by the caller before the panel is shown.
    /// </summary>
    public Tone? AutoRunTone { get; set; }

    private void OnToneClick(object? sender, EventArgs e)
    {
        if (sender is not WinForms.Button btn || btn.Tag is not Tone tone)
            return;
        RunTone(tone);
    }

    /// <summary>Selects a tone (highlights its button) and kicks off the
    /// rewrite — shared by manual clicks and the auto-run-on-open path.</summary>
    private void RunTone(Tone tone)
    {
        DRLogger.Log($"Tone selected: {tone}", DRLogger.Category.PANEL);
        UpdateToneButtonStyles(tone);

        if (ViewModel.RewriteCommand.CanExecute(tone))
            ViewModel.RewriteCommand.Execute(tone);
    }

    private void UpdateToneButtonStyles(Tone selected)
    {
        foreach (var (tone, btn) in _toneButtons)
        {
            if (tone == selected)
            {
                btn.BackColor = BrandBlue;
                btn.ForeColor = Color.White;
                btn.FlatAppearance.BorderColor = BrandBlue;
            }
            else
            {
                btn.BackColor = CardBg;
                btn.ForeColor = TextPrimary;
                btn.FlatAppearance.BorderColor = BorderColor;
            }
        }
    }

    private void OnViewModelPropertyChanged(object? sender, PropertyChangedEventArgs e)
    {
        // Marshal to UI thread if needed (the rewrite command runs async)
        if (InvokeRequired)
        {
            try { BeginInvoke(new Action(() => OnViewModelPropertyChanged(sender, e))); }
            catch (InvalidOperationException) { /* form may already be closing */ }
            return;
        }

        switch (e.PropertyName)
        {
            case nameof(RewritePanelViewModel.InputText):
                _inputBox.Text = ViewModel.InputText;
                break;
            case nameof(RewritePanelViewModel.OutputText):
                _outputBox.Text = ViewModel.OutputText;
                if (!string.IsNullOrEmpty(ViewModel.OutputText))
                    DRLogger.Log($"Rewrite success: {ViewModel.OutputText.Length} chars", DRLogger.Category.API);
                SyncReplaceCopyEnabled();
                break;
            case nameof(RewritePanelViewModel.UsageInfo):
                _usageLabel.Text = ViewModel.UsageInfo;
                break;
            case nameof(RewritePanelViewModel.ErrorMessage):
                var hasError = !string.IsNullOrEmpty(ViewModel.ErrorMessage);
                _errorLabel.Text = ViewModel.ErrorMessage;
                _errorLabel.Visible = hasError;
                _copyErrorBtn.Visible = hasError;
                if (hasError)
                    DRLogger.Error($"Rewrite error: {ViewModel.ErrorMessage}", DRLogger.Category.API);
                break;
            case nameof(RewritePanelViewModel.IsLoading):
                SyncLoadingState();
                break;
        }
    }

    private void SyncReplaceCopyEnabled()
    {
        var hasOutput = !string.IsNullOrEmpty(ViewModel.OutputText);
        _replaceBtn.Enabled = hasOutput;
        _copyBtn.Enabled = hasOutput;
        _copyBtn.Text = "Copy";
    }

    private void SyncLoadingState()
    {
        _loadingLabel.Visible = ViewModel.IsLoading;
        if (ViewModel.IsLoading) _loadingLabel.BringToFront();
        foreach (var btn in _toneButtons.Values) btn.Enabled = !ViewModel.IsLoading;
    }

    private void SafeClose()
    {
        try
        {
            if (InvokeRequired) BeginInvoke(new Action(() => Close()));
            else Close();
        }
        catch (InvalidOperationException)
        {
            // Form/handle already gone
        }
    }
}
