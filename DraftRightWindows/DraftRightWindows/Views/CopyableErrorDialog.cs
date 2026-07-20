using System;
using System.Drawing;
using System.IO;
using System.Threading;
using WinForms = System.Windows.Forms;

namespace DraftRightWindows.Views;

/// <summary>
/// Modal error dialog with selectable, multi-line text and a Copy-to-clipboard button.
/// Spins up its own STA thread so it works whether the caller is on the WinUI dispatcher,
/// the tray-icon thread, or a thread-pool worker.
/// </summary>
internal static class CopyableErrorDialog
{
    private static readonly Color BgDark = Color.FromArgb(15, 23, 42);
    private static readonly Color CardBg = Color.FromArgb(30, 41, 59);
    private static readonly Color BrandBlue = Color.FromArgb(93, 135, 255);
    private static readonly Color TextPrimary = Color.FromArgb(226, 232, 240);
    private static readonly Color BorderColor = Color.FromArgb(51, 65, 85);

    public static void Show(string title, string message)
    {
        var thread = new Thread(() => RunDialog(title, message));
        thread.SetApartmentState(ApartmentState.STA);
        thread.IsBackground = true;
        thread.Start();
    }

    private static void RunDialog(string title, string message)
    {
        var form = new WinForms.Form
        {
            Text = title,
            Width = 600,
            Height = 380,
            StartPosition = WinForms.FormStartPosition.CenterScreen,
            BackColor = BgDark,
            ForeColor = TextPrimary,
            FormBorderStyle = WinForms.FormBorderStyle.Sizable,
            MinimumSize = new Size(440, 260),
            ShowInTaskbar = true,
            TopMost = true,
        };

        // Embedded-resource load — survives single-file publish where the .ico
        // isn't next to the exe (#78). See Helpers/AppIcon.
        var ico = Helpers.AppIcon.Load();
        if (ico != null) form.Icon = ico;

        var textBox = new WinForms.TextBox
        {
            Multiline = true,
            ReadOnly = true,
            ScrollBars = WinForms.ScrollBars.Vertical,
            Text = message,
            Dock = WinForms.DockStyle.Fill,
            BackColor = CardBg,
            ForeColor = TextPrimary,
            BorderStyle = WinForms.BorderStyle.FixedSingle,
            Font = new Font("Consolas", 9.5f),
            WordWrap = true,
        };
        textBox.Select(0, 0);

        var buttonPanel = new WinForms.Panel
        {
            Dock = WinForms.DockStyle.Bottom,
            Height = 56,
            BackColor = BgDark,
        };

        var copyBtn = new WinForms.Button
        {
            Text = "Copy",
            Size = new Size(110, 32),
            BackColor = BrandBlue,
            ForeColor = Color.White,
            FlatStyle = WinForms.FlatStyle.Flat,
            Font = new Font("Segoe UI", 9.5f, FontStyle.Bold),
        };
        copyBtn.FlatAppearance.BorderSize = 0;
        copyBtn.Click += (_, _) =>
        {
            try
            {
                WinForms.Clipboard.SetText(message);
                copyBtn.Text = "Copied!";
            }
            catch
            {
                // Clipboard may be locked by another process; nothing actionable here.
            }
        };

        var closeBtn = new WinForms.Button
        {
            Text = "Close",
            Size = new Size(110, 32),
            BackColor = CardBg,
            ForeColor = TextPrimary,
            FlatStyle = WinForms.FlatStyle.Flat,
            Font = new Font("Segoe UI", 9.5f),
        };
        closeBtn.FlatAppearance.BorderColor = BorderColor;
        closeBtn.Click += (_, _) => form.Close();

        buttonPanel.Controls.Add(copyBtn);
        buttonPanel.Controls.Add(closeBtn);
        buttonPanel.Resize += (_, _) =>
        {
            closeBtn.Location = new Point(buttonPanel.Width - closeBtn.Width - 12, 12);
            copyBtn.Location = new Point(closeBtn.Left - copyBtn.Width - 8, 12);
        };

        form.Controls.Add(textBox);
        form.Controls.Add(buttonPanel);
        form.AcceptButton = closeBtn;

        WinForms.Application.Run(form);
    }
}
