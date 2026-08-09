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
    }
}
