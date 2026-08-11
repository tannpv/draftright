using System;
using System.Windows;
using System.Windows.Controls;
using System.Windows.Media;
using System.Windows.Media.Imaging;
using DraftRightWindows.Services;
using Wpf.Ui.Controls;

namespace DraftRightWindows.Views;

/// <summary>
/// Base class for every WPF-UI window in this app. It owns the two theme
/// invariants that a FluentWindow cannot render correctly without, so no
/// subclass has to remember them — and, more importantly, so they cannot be
/// forgotten as the WinForms→WPF migration (#156) adds one window at a time:
///
///   1. Merge WPF-UI's theme + control dictionaries per-window. This process
///      has no <c>App.xaml</c>, so nothing merges them globally; without them
///      controls resolve to no ControlTemplate and the window paints as a bare
///      white rectangle (#58).
///   2. Paint a solid, opaque dark background — and keep it. The Mica backdrop
///      makes the window background transparent to reveal the DWM effect, so an
///      opaque brush alone is NOT enough: on a Mica-capable Windows 11 with a
///      light wallpaper the panel renders translucent and the near-white text
///      washes out (#163). The backdrop is therefore forced OFF here so the
///      dark brush actually paints on every machine and wallpaper. On Windows
///      10 / VM / transparency-off there is no Mica anyway; this just makes the
///      result deterministic everywhere.
///
/// Both shipped as production regressions when they lived inline in a single
/// window and the next surface would have re-copied them. Centralising here is
/// the fix and the guard: RULE #1 — one source of truth for the theme contract.
/// Subclasses must NOT re-enable a backdrop — that reintroduces #163.
///
/// Subclasses set their own size and content in their constructor; the base
/// constructor runs first, so the dictionaries are already merged when the
/// subclass builds its controls.
/// </summary>
public abstract class FluentWindowBase : FluentWindow
{
    protected FluentWindowBase()
    {
        // (1) Control templates + theme brushes — see class remarks (#58).
        Resources.MergedDictionaries.Add(
            new Wpf.Ui.Markup.ThemesDictionary { Theme = Wpf.Ui.Appearance.ApplicationTheme.Dark });
        Resources.MergedDictionaries.Add(new Wpf.Ui.Markup.ControlsDictionary());

        // (2) No translucent backdrop — a Mica backdrop would blank our opaque
        // background and let the wallpaper bleed through, washing out the text
        // (#163). Deterministic readability beats the translucency effect.
        WindowBackdropType = WindowBackdropType.None;
        Background = Theme.WpfBrush(Theme.BgDark);

        // Render smoke-check (#164): fires once after the first paint. Every
        // window here is dark-themed, so a light average means the window failed
        // to render our theme — blank (#58), or washed-out because a backdrop
        // let the wallpaper through (#163). Report it to /errors so a broken
        // installed build surfaces from the field immediately, instead of only
        // via a user bug report. CI can't catch this: it compiles, it doesn't
        // render.
        ContentRendered += RenderSmokeCheck;
    }

    // Mean luminance above this (0–255) on a dark-themed window is treated as a
    // render failure. Correct windows sit near BgDark (~23); blank/white is ~240
    // and a washed-out light wallpaper ~150–200, both well above this.
    private const double LightMeanLuminance = 140;

    private bool _smokeChecked;

    private void RenderSmokeCheck(object? sender, EventArgs e)
    {
        if (_smokeChecked) return;
        _smokeChecked = true;
        ContentRendered -= RenderSmokeCheck;

        try
        {
            if (Content is not FrameworkElement root || root.ActualWidth < 1 || root.ActualHeight < 1)
            {
                ErrorReporter.Report(
                    new InvalidOperationException($"render-check: {GetType().Name} has zero-size content — window did not lay out (possible blank)."),
                    source: "render-check", severity: "warning");
                return;
            }

            // Downscale the window content into a tiny bitmap and average its
            // luminance — cheap, and enough to tell "dark theme rendered" from
            // "white/washed-out".
            const int w = 48, h = 48;
            var rtb = new RenderTargetBitmap(w, h, 96, 96, PixelFormats.Pbgra32);
            var dv = new DrawingVisual();
            using (var dc = dv.RenderOpen())
                dc.DrawRectangle(new VisualBrush(root), null, new Rect(0, 0, w, h));
            rtb.Render(dv);

            var px = new byte[w * h * 4];
            rtb.CopyPixels(px, w * 4, 0);
            double sum = 0;
            for (int i = 0; i < px.Length; i += 4) // BGRA
                sum += 0.299 * px[i + 2] + 0.587 * px[i + 1] + 0.114 * px[i];
            double meanLum = sum / (w * h);

            if (meanLum > LightMeanLuminance)
            {
                ErrorReporter.Report(
                    new InvalidOperationException($"render-check: {GetType().Name} rendered unexpectedly light (mean luminance {meanLum:F0}/255) — possible blank/unstyled/washed-out window."),
                    source: "render-check", severity: "warning");
            }
        }
        catch (Exception ex)
        {
            DRLogger.Warn($"render-check failed for {GetType().Name}: {ex.Message}", DRLogger.Category.APP);
        }
    }

    /// <summary>
    /// Sets the window content wrapped in a draggable WPF-UI title bar. A Fluent
    /// window drags by its <see cref="Wpf.Ui.Controls.TitleBar"/>, not the OS
    /// caption — a window without one is stuck where it opens (reported for the
    /// dialogs). Every window routes its content through here so the title bar
    /// (drag + minimise/close) is never forgotten. RULE #1: one chrome path.
    /// </summary>
    /// <param name="body">The window body, placed below the title bar.</param>
    /// <param name="showMaximize">Show the maximise button (true only for
    /// resizable windows; dialogs are fixed-size).</param>
    protected void SetBody(UIElement body, bool showMaximize = false)
    {
        ExtendsContentIntoTitleBar = true;

        var titleBar = new TitleBar
        {
            Title = Title,
            ShowMaximize = showMaximize,
        };
        DockPanel.SetDock(titleBar, Dock.Top);

        var root = new DockPanel { LastChildFill = true };
        root.Children.Add(titleBar);
        root.Children.Add(body);
        Content = root;
    }
}
