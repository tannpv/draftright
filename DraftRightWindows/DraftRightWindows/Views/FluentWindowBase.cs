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
///   2. Paint a solid dark background. The Mica backdrop only applies on
///      Windows 11 with transparency effects on; on Windows 10, in VMs/RDP, or
///      with transparency off it falls back to a light surface, and our
///      near-white text renders white-on-white (#163).
///
/// Both shipped as production regressions when they lived inline in a single
/// window and the next surface would have re-copied them. Centralising here is
/// the fix and the guard: RULE #1 — one source of truth for the theme contract.
///
/// Subclasses set their own size, backdrop, and content in their constructor;
/// the base constructor runs first, so the dictionaries are already merged when
/// the subclass builds its controls.
/// </summary>
public abstract class FluentWindowBase : FluentWindow
{
    protected FluentWindowBase()
    {
        // (1) Control templates + theme brushes — see class remarks (#58).
        Resources.MergedDictionaries.Add(
            new Wpf.Ui.Markup.ThemesDictionary { Theme = Wpf.Ui.Appearance.ApplicationTheme.Dark });
        Resources.MergedDictionaries.Add(new Wpf.Ui.Markup.ControlsDictionary());

        // (2) Opaque dark background so contrast holds regardless of whether the
        // Mica backdrop is available on this machine (#163). Paints over Mica
        // where supported; deterministic readability is worth more than the
        // translucency effect.
        Background = Theme.WpfBrush(Theme.BgDark);
    }
}
