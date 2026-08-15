using System;
using System.Drawing;
using System.Drawing.Drawing2D;
using WinForms = System.Windows.Forms;

namespace DraftRightWindows.Views;

/// <summary>
/// The floating pencil button shown next to a drag-selected piece of text
/// (Windows equivalent of the macOS pencil trigger, DraftRight #180). Unlike
/// <see cref="LoadingIndicator"/> (which is click-THROUGH) this one RECEIVES the
/// click, but still never activates — <c>WS_EX_NOACTIVATE | WS_EX_TOOLWINDOW</c>
/// plus <see cref="ShowWithoutActivation"/> keep the source app focused and its
/// selection intact, so a Ctrl+C on click still copies the user's selection.
///
/// Lives on its own STA message-pump thread owned by <see cref="Services.PencilTrigger"/>.
/// </summary>
internal sealed class PencilOverlayForm : WinForms.Form
{
    private const int Diameter = 30;
    private static readonly Color Blue = Color.FromArgb(255, 93, 135, 255);

    public event Action? PencilClicked;

    public PencilOverlayForm()
    {
        FormBorderStyle = WinForms.FormBorderStyle.None;
        ShowInTaskbar = false;
        TopMost = true;
        StartPosition = WinForms.FormStartPosition.Manual;
        Size = new Size(Diameter, Diameter);
        BackColor = Color.Magenta;         // keyed out → round button on transparent bg
        TransparencyKey = Color.Magenta;
        DoubleBuffered = true;
        Cursor = WinForms.Cursors.Hand;

        Paint += OnPaint;
        Click += (_, _) => PencilClicked?.Invoke();
    }

    protected override WinForms.CreateParams CreateParams
    {
        get
        {
            var cp = base.CreateParams;
            // WS_EX_TOOLWINDOW = 0x80, WS_EX_NOACTIVATE = 0x08000000.
            // No WS_EX_TRANSPARENT here — we WANT the click.
            cp.ExStyle |= 0x80 | 0x08000000;
            return cp;
        }
    }

    protected override bool ShowWithoutActivation => true;

    /// <summary>Show the button just below-right of a screen point (the drag end).</summary>
    public void ShowAt(Point screenPoint)
    {
        Location = new Point(screenPoint.X + 8, screenPoint.Y + 12);
        if (!Visible) Show();
        else BringToFront();
    }

    private void OnPaint(object? sender, WinForms.PaintEventArgs e)
    {
        var g = e.Graphics;
        g.SmoothingMode = SmoothingMode.AntiAlias;
        using var bg = new SolidBrush(Blue);
        g.FillEllipse(bg, new Rectangle(0, 0, Diameter - 1, Diameter - 1));
        using var fg = new SolidBrush(Color.White);
        using var font = new Font("Segoe UI Symbol", 13f, FontStyle.Bold);
        const string glyph = "✎"; // LOWER RIGHT PENCIL
        var sz = g.MeasureString(glyph, font);
        g.DrawString(glyph, font, fg, new PointF((Diameter - sz.Width) / 2f, (Diameter - sz.Height) / 2f));
    }
}
