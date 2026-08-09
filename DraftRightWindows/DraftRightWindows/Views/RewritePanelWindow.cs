using System;
using System.Collections.Generic;
using System.ComponentModel;
using System.Windows;
using System.Windows.Controls;
using System.Windows.Controls.Primitives;
using System.Windows.Documents;
using DraftRightWindows.Diff;
using DraftRightWindows.Models;
using DraftRightWindows.Services;
using DraftRightWindows.ViewModels;
using Wpf.Ui.Controls;
using WpfButton = Wpf.Ui.Controls.Button;
using WpfTextBlock = System.Windows.Controls.TextBlock;

namespace DraftRightWindows.Views;

/// <summary>
/// The rewrite panel in WPF UI — first surface migrated off WinForms (#156).
///
/// Binds to the same <see cref="RewritePanelViewModel"/> the WinForms panel
/// used. No logic moves: the view renders state and raises commands, exactly
/// as before. That is what makes the migration safe to do one panel at a time.
///
/// Built in C# rather than XAML markup to match the rest of this project,
/// which has no XAML files and therefore none of the build machinery they
/// require. WPF-UI's theme dictionaries are merged per-window, since this
/// process has no App.xaml — that omission is what made the spike render
/// blank (bug report #58).
/// </summary>
public sealed class RewritePanelWindow : FluentWindow
{
    public RewritePanelViewModel ViewModel { get; } = new();

    /// <summary>
    /// Tone to run automatically once the panel is up, so a configured
    /// default produces a rewrite without a click. Null means manual pick.
    /// Set before showing; fires on Loaded rather than in the constructor so
    /// the spinner and buttons already exist to reflect it.
    /// </summary>
    public Tone? AutoRunTone { get; set; }

    private readonly System.Windows.Controls.TextBox _inputBox;
    private readonly WpfTextBlock _outputBlock;
    private readonly ScrollViewer _outputScroll;
    private readonly GrammarIssuesView _grammarView;
    private readonly DiffView _diffView;
    private readonly ProgressRing _spinner;
    private readonly WpfTextBlock _usageText;
    private readonly InfoBar _errorBar;
    private readonly WpfButton _replaceBtn;
    private readonly WpfButton _copyBtn;
    private readonly WpfButton _diffBtn;
    private readonly Dictionary<Tone, WpfButton> _toneButtons = new();

    public RewritePanelWindow()
    {
        Title = "DraftRight";
        Width = 560;
        Height = 660;
        WindowStartupLocation = WindowStartupLocation.CenterScreen;
        WindowBackdropType = WindowBackdropType.Mica;
        ExtendsContentIntoTitleBar = true;
        ResizeMode = ResizeMode.CanResize;

        // Solid dark fallback background. The Mica backdrop only paints on
        // Windows 11 with transparency effects on; on Windows 10, in VMs/RDP,
        // or with transparency off it does not apply and the FluentWindow falls
        // back to a light system surface. Our foreground text is near-white
        // (Theme.TextPrimary), so without an explicit dark background it renders
        // white-on-white and the panel looks empty (#163). An opaque dark
        // background guarantees contrast on every config. It paints over Mica
        // where Mica is supported, but deterministic readability is worth more
        // than the translucency effect.
        Background = Theme.WpfBrush(Theme.BgDark);

        // No App.xaml in this process, so nothing merges WPF-UI's control
        // templates or theme brushes globally. Without these the window paints
        // as a bare white rectangle (#58).
        Resources.MergedDictionaries.Add(
            new Wpf.Ui.Markup.ThemesDictionary { Theme = Wpf.Ui.Appearance.ApplicationTheme.Dark });
        Resources.MergedDictionaries.Add(new Wpf.Ui.Markup.ControlsDictionary());

        var root = new Grid { Margin = new Thickness(16, 0, 16, 16) };
        foreach (var h in new[] { GridLength.Auto, GridLength.Auto, GridLength.Auto,
                                  new GridLength(1, GridUnitType.Star), GridLength.Auto,
                                  GridLength.Auto, GridLength.Auto })
        {
            root.RowDefinitions.Add(new RowDefinition { Height = h });
        }

        var titleBar = new TitleBar { Title = "DraftRight" };
        Grid.SetRow(titleBar, 0);
        root.Children.Add(titleBar);

        // ── Captured text ──
        _inputBox = new System.Windows.Controls.TextBox
        {
            IsReadOnly = true,
            TextWrapping = TextWrapping.Wrap,
            VerticalScrollBarVisibility = ScrollBarVisibility.Auto,
            Height = 72,
            Margin = new Thickness(0, 8, 0, 12),
        };
        Grid.SetRow(_inputBox, 1);
        root.Children.Add(_inputBox);

        // ── Tones ──
        var toneGrid = BuildToneGrid();
        Grid.SetRow(toneGrid, 2);
        root.Children.Add(toneGrid);

        // ── Output: text / grammar / diff, one visible at a time ──
        var outputHost = new Grid { Margin = new Thickness(0, 12, 0, 8) };

        _outputBlock = new WpfTextBlock
        {
            TextWrapping = TextWrapping.Wrap,
            Foreground = Theme.WpfBrush(Theme.TextPrimary),
            Margin = new Thickness(12),
        };
        _outputScroll = new ScrollViewer
        {
            Content = _outputBlock,
            VerticalScrollBarVisibility = ScrollBarVisibility.Auto,
        };
        outputHost.Children.Add(_outputScroll);

        _grammarView = new GrammarIssuesView { Visibility = Visibility.Collapsed };
        _grammarView.TextChanged += (_, corrected) => ViewModel.OutputText = corrected;
        outputHost.Children.Add(_grammarView);

        _diffView = new DiffView { Visibility = Visibility.Collapsed };
        outputHost.Children.Add(_diffView);

        _spinner = new ProgressRing
        {
            IsIndeterminate = true,
            Width = 40,
            Height = 40,
            HorizontalAlignment = HorizontalAlignment.Center,
            VerticalAlignment = VerticalAlignment.Center,
            Visibility = Visibility.Collapsed,
        };
        outputHost.Children.Add(_spinner);

        Grid.SetRow(outputHost, 3);
        root.Children.Add(outputHost);

        // ── Usage ──
        _usageText = new WpfTextBlock
        {
            Foreground = Theme.WpfBrush(Theme.TextMuted),
            FontSize = 12,
            Margin = new Thickness(2, 0, 0, 8),
        };
        Grid.SetRow(_usageText, 4);
        root.Children.Add(_usageText);

        // ── Errors ──
        _errorBar = new InfoBar
        {
            Severity = InfoBarSeverity.Error,
            IsClosable = true,
            IsOpen = false,
            Margin = new Thickness(0, 0, 0, 8),
        };
        Grid.SetRow(_errorBar, 5);
        root.Children.Add(_errorBar);

        // ── Actions ──
        var actions = new StackPanel
        {
            Orientation = System.Windows.Controls.Orientation.Horizontal,
            HorizontalAlignment = HorizontalAlignment.Right,
        };
        _diffBtn = MakeAction("Diff", ControlAppearance.Secondary, ToggleDiff);
        _diffBtn.Visibility = Visibility.Collapsed;
        _copyBtn = MakeAction("Copy", ControlAppearance.Secondary,
            () => ViewModel.CopyCommand.Execute(null));
        _replaceBtn = MakeAction("Replace", ControlAppearance.Primary,
            () => ViewModel.ReplaceCommand.Execute(null));
        var closeBtn = MakeAction("Close", ControlAppearance.Secondary,
            () => ViewModel.CloseCommand.Execute(null));
        foreach (var b in new[] { _diffBtn, closeBtn, _copyBtn, _replaceBtn }) actions.Children.Add(b);
        Grid.SetRow(actions, 6);
        root.Children.Add(actions);

        Content = root;

        ViewModel.PropertyChanged += OnViewModelPropertyChanged;
        ViewModel.CloseRequested += (_, _) => Dispatcher.Invoke(Close);
        Closed += (_, _) => ViewModel.PropertyChanged -= OnViewModelPropertyChanged;

        Loaded += (_, _) =>
        {
            if (AutoRunTone is Tone tone) ViewModel.RewriteCommand.Execute(tone);
        };

        SyncActionsEnabled();
    }

    /// <summary>Sets the captured text. Call before showing.</summary>
    public void SetInputText(string text) => ViewModel.InputText = text ?? string.Empty;

    // ── Building blocks ──

    /// <summary>
    /// Tone buttons: every tone the user has enabled in Settings.
    ///
    /// The WinForms panel carried its own array restating each tone's icon and
    /// label — values <see cref="ToneExtensions"/> already owns — and that
    /// array quietly became the real definition of which tones exist. It
    /// listed six, and it consulted <c>Settings.EnabledTones</c> not at all.
    ///
    /// So the per-tone toggles in Settings did nothing for this panel, and
    /// Claude and Grammar Check — both enabled by default — were unreachable.
    /// The grammar results view built for #107 could never be shown at all.
    ///
    /// Reading the enum for the buttons and Settings for the filter fixes the
    /// duplication and the dead setting together: add a tone to the enum and
    /// it appears here, tick it in Settings and it shows up (Rule #1).
    /// </summary>
    private UniformGrid BuildToneGrid()
    {
        var grid = new UniformGrid { Columns = 4, Margin = new Thickness(0, 0, 0, 4) };
        foreach (var tone in ViewModel.Tones)
        {
            if (!App.Settings.EnabledTones.Contains(tone.ApiValue())) continue;
            var btn = new WpfButton
            {
                Content = $"{tone.Icon()}  {tone.DisplayName()}",
                Appearance = ControlAppearance.Secondary,
                HorizontalAlignment = HorizontalAlignment.Stretch,
                Margin = new Thickness(2),
                FontSize = 12,
            };
            var captured = tone;
            btn.Click += (_, _) => ViewModel.RewriteCommand.Execute(captured);
            _toneButtons[tone] = btn;
            grid.Children.Add(btn);
        }
        return grid;
    }

    private WpfButton MakeAction(string label, ControlAppearance appearance, Action onClick)
    {
        var btn = new WpfButton
        {
            Content = label,
            Appearance = appearance,
            MinWidth = 88,
            Margin = new Thickness(6, 0, 0, 0),
        };
        btn.Click += (_, _) => onClick();
        return btn;
    }

    // ── ViewModel → view ──

    private void OnViewModelPropertyChanged(object? sender, PropertyChangedEventArgs e)
    {
        if (!Dispatcher.CheckAccess())
        {
            Dispatcher.Invoke(() => OnViewModelPropertyChanged(sender, e));
            return;
        }

        switch (e.PropertyName)
        {
            case nameof(RewritePanelViewModel.InputText):
                _inputBox.Text = ViewModel.InputText;
                break;
            case nameof(RewritePanelViewModel.OutputText):
                _outputBlock.Text = ViewModel.OutputText;
                SyncActionsEnabled();
                SyncDiffAvailability();
                break;
            case nameof(RewritePanelViewModel.UsageInfo):
                _usageText.Text = ViewModel.UsageInfo;
                break;
            case nameof(RewritePanelViewModel.ErrorMessage):
                var hasError = !string.IsNullOrEmpty(ViewModel.ErrorMessage);
                _errorBar.Message = ViewModel.ErrorMessage;
                _errorBar.IsOpen = hasError;
                break;
            case nameof(RewritePanelViewModel.IsLoading):
                _spinner.Visibility = ViewModel.IsLoading ? Visibility.Visible : Visibility.Collapsed;
                foreach (var b in _toneButtons.Values) b.IsEnabled = !ViewModel.IsLoading;
                break;
            case nameof(RewritePanelViewModel.SelectedTone):
                SyncToneSelection();
                break;
            case nameof(RewritePanelViewModel.GrammarResult):
                SyncGrammarResult();
                break;
        }
    }

    private void SyncToneSelection()
    {
        foreach (var (tone, btn) in _toneButtons)
        {
            btn.Appearance = tone == ViewModel.SelectedTone
                ? ControlAppearance.Primary
                : ControlAppearance.Secondary;
        }
    }

    private void SyncActionsEnabled()
    {
        var has = !string.IsNullOrWhiteSpace(ViewModel.OutputText);
        _replaceBtn.IsEnabled = has;
        _copyBtn.IsEnabled = has;
    }

    /// <summary>Exactly one of text / grammar / diff is ever visible.</summary>
    private void ShowOutput(OutputMode mode)
    {
        _outputScroll.Visibility = mode == OutputMode.Text ? Visibility.Visible : Visibility.Collapsed;
        _grammarView.Visibility = mode == OutputMode.Grammar ? Visibility.Visible : Visibility.Collapsed;
        _diffView.Visibility = mode == OutputMode.Diff ? Visibility.Visible : Visibility.Collapsed;
    }

    private enum OutputMode { Text, Grammar, Diff }

    private void SyncGrammarResult()
    {
        var result = ViewModel.GrammarResult;
        if (result == null)
        {
            ShowOutput(OutputMode.Text);
            return;
        }
        // Issues resolve against the INPUT — the text they were found in, and
        // what a fix applies to.
        _grammarView.Load(ViewModel.InputText, result);
        ShowOutput(OutputMode.Grammar);
    }

    private void SyncDiffAvailability()
    {
        var canDiff = !string.IsNullOrEmpty(ViewModel.InputText)
                      && !string.IsNullOrEmpty(ViewModel.OutputText);
        _diffBtn.Visibility = canDiff ? Visibility.Visible : Visibility.Collapsed;
        if (!canDiff && _diffView.Visibility == Visibility.Visible) ShowOutput(OutputMode.Text);
    }

    private void ToggleDiff()
    {
        if (_diffView.Visibility == Visibility.Visible)
        {
            ShowOutput(ViewModel.GrammarResult != null ? OutputMode.Grammar : OutputMode.Text);
            _diffBtn.Content = "Diff";
            return;
        }
        _diffView.ShowDiff(ViewModel.InputText, ViewModel.OutputText);
        ShowOutput(OutputMode.Diff);
        _diffBtn.Content = "Hide diff";
    }
}
