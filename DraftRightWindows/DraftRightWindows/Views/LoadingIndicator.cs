using System;
using System.Drawing;
using System.Drawing.Drawing2D;
using WinForms = System.Windows.Forms;

namespace DraftRightWindows.Views;

/// <summary>
/// A 36x36 always-on-top, click-through window with a pulsing pencil icon.
/// Shown while a One-Click rewrite is in flight. Uses a WinForms timer to
/// animate opacity between 1.0 and 0.25.
/// </summary>
public sealed class LoadingIndicator : WinForms.Form
{
    private readonly WinForms.Timer _timer;
    private float _phase; // 0..2π
    private const float TwoPi = (float)(Math.PI * 2);
    private const int Size = 36;

    public LoadingIndicator()
    {
        FormBorderStyle = WinForms.FormBorderStyle.None;
        StartPosition = WinForms.FormStartPosition.Manual;
        ShowInTaskbar = false;
        TopMost = true;
        Width = Size;
        Height = Size;
        BackColor = Color.Magenta; // treated as transparent via TransparencyKey
        TransparencyKey = Color.Magenta;
        DoubleBuffered = true;

        // Click-through: WS_EX_TRANSPARENT | WS_EX_LAYERED | WS_EX_TOOLWINDOW
        // Prevents the indicator from stealing mouse focus from the user's target app.
        // WS_EX_NOACTIVATE keeps it from stealing keyboard focus either.

        _timer = new WinForms.Timer { Interval = 50 };
        _timer.Tick += OnTick;

        Paint += OnPaint;
    }

    protected override WinForms.CreateParams CreateParams
    {
        get
        {
            var cp = base.CreateParams;
            // WS_EX_TOOLWINDOW = 0x80, WS_EX_NOACTIVATE = 0x08000000,
            // WS_EX_TRANSPARENT = 0x20, WS_EX_LAYERED = 0x80000
            cp.ExStyle |= 0x80 | 0x08000000 | 0x20 | 0x80000;
            return cp;
        }
    }

    protected override bool ShowWithoutActivation => true;

    public void ShowAtCursor()
    {
        var pos = WinForms.Cursor.Position;
        Left = pos.X + 12;
        Top = pos.Y + 12;
        _phase = 0f;
        _timer.Start();
        Show();
    }

    public void HideIndicator()
    {
        _timer.Stop();
        Hide();
    }

    private void OnTick(object? sender, EventArgs e)
    {
        // Advance phase: one full sin cycle ≈ 1.1s
        // 2π / 0.28 ≈ 22.4 ticks × 50ms = 1.12s per cycle
        _phase += 0.28f;
        if (_phase > TwoPi) _phase -= TwoPi;
        Invalidate();
    }

    private void OnPaint(object? sender, WinForms.PaintEventArgs e)
    {
        var g = e.Graphics;
        g.SmoothingMode = SmoothingMode.AntiAlias;

        // Compute pulse opacity between 0.25 and 1.0
        // cos(_phase) ranges [-1, 1] → pulse ranges [0.25, 1.0]
        float pulse = 0.625f + 0.375f * (float)Math.Cos(_phase);
        int alpha = Math.Clamp((int)(pulse * 255f), 64, 255);

        // Blue circle background (always fully opaque — only the pencil glyph pulses)
        var circleRect = new Rectangle(0, 0, Size - 1, Size - 1);
        using var bgBrush = new SolidBrush(Color.FromArgb(255, 93, 135, 255));
        g.FillEllipse(bgBrush, circleRect);

        // Pencil icon as Unicode glyph, centered, with pulsing alpha
        using var fgBrush = new SolidBrush(Color.FromArgb(alpha, 255, 255, 255));
        using var font = new Font("Segoe UI Symbol", 14f, FontStyle.Bold);
        var glyph = "\u270E"; // LOWER RIGHT PENCIL
        var glyphSize = g.MeasureString(glyph, font);
        var glyphPos = new PointF((Size - glyphSize.Width) / 2f, (Size - glyphSize.Height) / 2f);
        g.DrawString(glyph, font, fgBrush, glyphPos);
    }
}
