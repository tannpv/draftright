using System;
using System.Threading;
using WinForms = System.Windows.Forms;
using System.Drawing;

namespace DraftRightWindows.Views;

/// <summary>
/// One-time "What's New in DraftRight vX" notice shown on the first launch
/// after an update applies. Runs on its own STA thread with a WinForms message
/// pump (Application.Run) — same approach as the updater progress window — so
/// it paints reliably regardless of which thread asked to show it.
/// </summary>
public static class WhatsNewWindow
{
    /// <summary>Shows the notice on a dedicated STA thread and returns
    /// immediately. <paramref name="notes"/> is the release-note text from the
    /// backend (markdown-ish bullet lines); it's displayed verbatim.</summary>
    public static void Show(string version, string notes)
    {
        var thread = new Thread(() =>
        {
            try { WinForms.Application.Run(BuildForm(version, notes)); }
            catch { /* never let a notice window crash the app */ }
        });
        thread.SetApartmentState(ApartmentState.STA);
        thread.IsBackground = true;
        thread.Start();
    }

    private static WinForms.Form BuildForm(string version, string notes)
    {
        var form = new WinForms.Form
        {
            Text = "What's New",
            Width = 460,
            Height = 360,
            StartPosition = WinForms.FormStartPosition.CenterScreen,
            FormBorderStyle = WinForms.FormBorderStyle.FixedDialog,
            MaximizeBox = false,
            MinimizeBox = false,
            TopMost = true,
            BackColor = Color.FromArgb(15, 23, 42),
            ForeColor = Color.FromArgb(226, 232, 240),
        };

        var heading = new WinForms.Label
        {
            Text = $"What's new in DraftRight v{version}",
            Location = new Point(20, 18),
            Size = new Size(420, 26),
            Font = new Font("Segoe UI", 12, FontStyle.Bold),
            ForeColor = Color.FromArgb(226, 232, 240),
        };

        var body = new WinForms.TextBox
        {
            Text = string.IsNullOrWhiteSpace(notes) ? "Various improvements and fixes." : notes,
            Location = new Point(20, 54),
            Size = new Size(420, 220),
            Multiline = true,
            ReadOnly = true,
            ScrollBars = WinForms.ScrollBars.Vertical,
            BorderStyle = WinForms.BorderStyle.FixedSingle,
            BackColor = Color.FromArgb(30, 41, 59),
            ForeColor = Color.FromArgb(226, 232, 240),
            Font = new Font("Segoe UI", 10),
            TabStop = false,
        };
        // Start at the top, no selection highlight.
        body.Select(0, 0);

        var closeButton = new WinForms.Button
        {
            Text = "Got it",
            Location = new Point(350, 286),
            Size = new Size(90, 32),
            FlatStyle = WinForms.FlatStyle.Flat,
            BackColor = Color.FromArgb(59, 130, 246),
            ForeColor = Color.White,
            Font = new Font("Segoe UI", 9, FontStyle.Bold),
            UseVisualStyleBackColor = false,
            Cursor = WinForms.Cursors.Hand,
        };
        closeButton.FlatAppearance.BorderSize = 0;
        closeButton.Click += (_, _) => form.Close();

        form.Controls.AddRange(new WinForms.Control[] { heading, body, closeButton });
        form.AcceptButton = closeButton;
        return form;
    }
}
