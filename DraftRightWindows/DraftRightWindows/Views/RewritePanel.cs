using System;
using System.Collections.Generic;
using Microsoft.UI;
using Microsoft.UI.Xaml;
using Microsoft.UI.Xaml.Controls;
using Microsoft.UI.Xaml.Media;
using DraftRightWindows.Models;
using DraftRightWindows.Services;
using DraftRightWindows.ViewModels;

namespace DraftRightWindows.Views;

public sealed class RewritePanel : Window
{
    public RewritePanelViewModel ViewModel { get; }

    // Map tone button names to the Tone enum
    private static readonly Dictionary<string, Tone> ToneMap = new()
    {
        ["Simple"] = Tone.Simple,
        ["Natural"] = Tone.Natural,
        ["Polished"] = Tone.Polished,
        ["Concise"] = Tone.Concise,
        ["Technical"] = Tone.Technical,
        ["Translate"] = Tone.Translate,
    };

    // Keep references to all tone buttons for styling updates
    private readonly Dictionary<string, Button> _toneButtons = new();

    // UI element references
    private TextBlock _inputTextBlock = null!;
    private TextBlock _outputTextBlock = null!;
    private TextBlock _usageTextBlock = null!;
    private TextBlock _errorTextBlock = null!;
    private Grid _loadingOverlay = null!;
    private ProgressRing _loadingRing = null!;
    private Border _outputBorder = null!;          // wraps _outputTextBlock — toggled when grammar UI is shown
    private ContentControl _grammarHost = null!;    // hosts GrammarCheckView when tone == grammar_check

    // Colors
    private static readonly SolidColorBrush BrandBlue = new(Windows.UI.Color.FromArgb(255, 93, 135, 255));
    private static readonly SolidColorBrush CardBg = new(Windows.UI.Color.FromArgb(255, 30, 41, 59));
    private static readonly SolidColorBrush BorderColor = new(Windows.UI.Color.FromArgb(255, 51, 65, 85));
    private static readonly SolidColorBrush TextPrimary = new(Windows.UI.Color.FromArgb(255, 226, 232, 240));
    private static readonly SolidColorBrush TextMuted = new(Windows.UI.Color.FromArgb(255, 148, 163, 184));
    private static readonly SolidColorBrush ErrorRed = new(Windows.UI.Color.FromArgb(255, 239, 68, 68));
    private static readonly SolidColorBrush ResultBg = new(Windows.UI.Color.FromArgb(255, 15, 41, 34));
    private static readonly SolidColorBrush Background = new(Windows.UI.Color.FromArgb(255, 15, 23, 42));
    private static readonly SolidColorBrush LoadingOverlayBg = new(Windows.UI.Color.FromArgb(136, 15, 23, 42));

    public RewritePanel()
    {
        ViewModel = new RewritePanelViewModel();

        // Configure window. Note: ExtendsContentIntoTitleBar=true combined
        // with SetTitleBar(null) crashes Microsoft.UI.Xaml.dll on ARM64 first
        // render. Use the default chrome until we have a proper custom title
        // bar element to set.
        Title = "DraftRight";

        // Build the UI
        Content = BuildUI();

        // Subscribe to close request
        ViewModel.CloseRequested += (_, _) => this.Close();

        // Subscribe to property changes for manual bindings
        ViewModel.PropertyChanged += OnViewModelPropertyChanged;
    }

    private Grid BuildUI()
    {
        var root = new Grid
        {
            Background = Background,
            Padding = new Thickness(16),
        };

        // Row definitions
        root.RowDefinitions.Add(new RowDefinition { Height = GridLength.Auto });  // Input preview
        root.RowDefinitions.Add(new RowDefinition { Height = GridLength.Auto });  // Tone grid
        root.RowDefinitions.Add(new RowDefinition { Height = new GridLength(1, GridUnitType.Star) }); // Result area
        root.RowDefinitions.Add(new RowDefinition { Height = GridLength.Auto });  // Usage info
        root.RowDefinitions.Add(new RowDefinition { Height = GridLength.Auto });  // Action buttons
        root.RowDefinitions.Add(new RowDefinition { Height = GridLength.Auto });  // Error message

        // Row 0: Input text preview
        root.Children.Add(BuildInputPreview());

        // Row 1: Tone button grid
        root.Children.Add(BuildToneGrid());

        // Row 2: Result area with loading overlay
        root.Children.Add(BuildResultArea());

        // Row 3: Usage info
        root.Children.Add(BuildUsageInfo());

        // Row 4: Action buttons
        root.Children.Add(BuildActionButtons());

        // Row 5: Error message
        root.Children.Add(BuildErrorMessage());

        return root;
    }

    private Border BuildInputPreview()
    {
        _inputTextBlock = new TextBlock
        {
            Foreground = TextMuted,
            FontSize = 13,
            MaxLines = 3,
            TextTrimming = TextTrimming.CharacterEllipsis,
            TextWrapping = TextWrapping.Wrap,
        };

        var border = new Border
        {
            Background = CardBg,
            BorderBrush = BorderColor,
            BorderThickness = new Thickness(1),
            CornerRadius = new CornerRadius(8),
            Padding = new Thickness(12, 10, 12, 10),
            Margin = new Thickness(0, 0, 0, 12),
            Child = _inputTextBlock,
        };

        Grid.SetRow(border, 0);
        return border;
    }

    private Grid BuildToneGrid()
    {
        var grid = new Grid
        {
            Margin = new Thickness(0, 0, 0, 12),
            ColumnSpacing = 8,
            RowSpacing = 8,
        };

        grid.ColumnDefinitions.Add(new ColumnDefinition { Width = new GridLength(1, GridUnitType.Star) });
        grid.ColumnDefinitions.Add(new ColumnDefinition { Width = new GridLength(1, GridUnitType.Star) });
        grid.ColumnDefinitions.Add(new ColumnDefinition { Width = new GridLength(1, GridUnitType.Star) });
        grid.RowDefinitions.Add(new RowDefinition { Height = GridLength.Auto });
        grid.RowDefinitions.Add(new RowDefinition { Height = GridLength.Auto });

        var tones = new (string Name, string Emoji, int Row, int Col)[]
        {
            ("Simple",    "\u270E",  0, 0),
            ("Natural",   "\uD83D\uDCAC", 0, 1),
            ("Polished",  "\u2728",  0, 2),
            ("Concise",   "\u2296",  1, 0),
            ("Technical", "\uD83D\uDD27", 1, 1),
            ("Translate", "\uD83C\uDF10", 1, 2),
        };

        foreach (var (name, emoji, row, col) in tones)
        {
            var btn = CreateToneButton(name, emoji);
            Grid.SetRow(btn, row);
            Grid.SetColumn(btn, col);
            grid.Children.Add(btn);
            _toneButtons[name] = btn;
        }

        Grid.SetRow(grid, 1);
        return grid;
    }

    private Button CreateToneButton(string toneName, string emoji)
    {
        var sp = new StackPanel
        {
            Orientation = Orientation.Horizontal,
            Spacing = 4,
        };
        sp.Children.Add(new TextBlock { Text = emoji, FontSize = 14 });
        sp.Children.Add(new TextBlock { Text = toneName, FontSize = 13 });

        var btn = new Button
        {
            Tag = toneName,
            Content = sp,
            HorizontalAlignment = HorizontalAlignment.Stretch,
            HorizontalContentAlignment = HorizontalAlignment.Center,
        };

        ApplyToneButtonStyle(btn, false);
        btn.Click += OnToneClick;
        return btn;
    }

    private static void ApplyToneButtonStyle(Button btn, bool selected)
    {
        if (selected)
        {
            btn.Background = BrandBlue;
            btn.Foreground = new SolidColorBrush(Colors.White);
            btn.BorderBrush = BrandBlue;
        }
        else
        {
            btn.Background = CardBg;
            btn.Foreground = TextPrimary;
            btn.BorderBrush = BorderColor;
        }

        btn.BorderThickness = new Thickness(1);
        btn.CornerRadius = new CornerRadius(8);
        btn.Padding = new Thickness(8, 10, 8, 10);
    }

    private Grid BuildResultArea()
    {
        var container = new Grid
        {
            Margin = new Thickness(0, 0, 0, 8),
        };

        // Result text
        _outputTextBlock = new TextBlock
        {
            Foreground = TextPrimary,
            FontSize = 14,
            TextWrapping = TextWrapping.Wrap,
            IsTextSelectionEnabled = true,
        };

        var scrollViewer = new ScrollViewer
        {
            VerticalScrollBarVisibility = ScrollBarVisibility.Auto,
            Content = _outputTextBlock,
        };

        _outputBorder = new Border
        {
            Background = ResultBg,
            BorderBrush = BorderColor,
            BorderThickness = new Thickness(1),
            CornerRadius = new CornerRadius(8),
            Padding = new Thickness(12),
            Child = scrollViewer,
        };
        container.Children.Add(_outputBorder);

        // GrammarCheckView host disabled while debugging WinUI crash on ARM64.
        // The empty ContentControl was causing a render-frame crash inside
        // Microsoft.UI.Xaml.dll right after Window activation. Re-add once the
        // root cause is identified.
        _grammarHost = null!;

        // Loading overlay
        _loadingRing = new ProgressRing
        {
            IsActive = false,
            Width = 36,
            Height = 36,
            Foreground = BrandBlue,
        };

        _loadingOverlay = new Grid
        {
            Background = LoadingOverlayBg,
            Visibility = Visibility.Collapsed,
        };
        _loadingOverlay.Children.Add(_loadingRing);

        container.Children.Add(_loadingOverlay);

        Grid.SetRow(container, 2);
        return container;
    }

    private TextBlock BuildUsageInfo()
    {
        _usageTextBlock = new TextBlock
        {
            Foreground = TextMuted,
            FontSize = 12,
            Margin = new Thickness(0, 0, 0, 8),
            HorizontalAlignment = HorizontalAlignment.Right,
        };

        Grid.SetRow(_usageTextBlock, 3);
        return _usageTextBlock;
    }

    private StackPanel BuildActionButtons()
    {
        var panel = new StackPanel
        {
            Orientation = Orientation.Horizontal,
            HorizontalAlignment = HorizontalAlignment.Right,
            Spacing = 8,
            Margin = new Thickness(0, 0, 0, 4),
        };

        // Replace button (primary)
        var replaceBtn = new Button
        {
            Content = "Replace",
            Background = BrandBlue,
            Foreground = new SolidColorBrush(Colors.White),
            CornerRadius = new CornerRadius(6),
            Padding = new Thickness(16, 8, 16, 8),
            BorderThickness = new Thickness(0),
        };
        replaceBtn.Command = ViewModel.ReplaceCommand;
        panel.Children.Add(replaceBtn);

        // Copy button (outlined)
        var copyBtn = new Button
        {
            Content = "Copy",
            Background = new SolidColorBrush(Colors.Transparent),
            Foreground = TextPrimary,
            BorderBrush = BorderColor,
            BorderThickness = new Thickness(1),
            CornerRadius = new CornerRadius(6),
            Padding = new Thickness(16, 8, 16, 8),
        };
        copyBtn.Command = ViewModel.CopyCommand;
        panel.Children.Add(copyBtn);

        // Close button (ghost)
        var closeBtn = new Button
        {
            Content = "Close",
            Background = new SolidColorBrush(Colors.Transparent),
            Foreground = TextMuted,
            BorderThickness = new Thickness(0),
            CornerRadius = new CornerRadius(6),
            Padding = new Thickness(16, 8, 16, 8),
        };
        closeBtn.Command = ViewModel.CloseCommand;
        panel.Children.Add(closeBtn);

        Grid.SetRow(panel, 4);
        return panel;
    }

    private StackPanel BuildErrorMessage()
    {
        var panel = new StackPanel
        {
            Orientation = Orientation.Horizontal,
            Spacing = 8,
            Margin = new Thickness(0, 4, 0, 0),
        };

        _errorTextBlock = new TextBlock
        {
            Foreground = ErrorRed,
            FontSize = 12,
            TextWrapping = TextWrapping.Wrap,
            IsTextSelectionEnabled = true,
            VerticalAlignment = VerticalAlignment.Center,
        };
        panel.Children.Add(_errorTextBlock);

        var copyErrorBtn = new Button
        {
            Content = "Copy",
            FontSize = 11,
            Padding = new Thickness(8, 4, 8, 4),
            Background = new SolidColorBrush(Colors.Transparent),
            Foreground = ErrorRed,
            BorderBrush = ErrorRed,
            BorderThickness = new Thickness(1),
            CornerRadius = new CornerRadius(4),
            VerticalAlignment = VerticalAlignment.Center,
            Visibility = Visibility.Collapsed,
        };
        copyErrorBtn.Click += (_, _) =>
        {
            var dp = new Windows.ApplicationModel.DataTransfer.DataPackage();
            dp.SetText(_errorTextBlock.Text);
            Windows.ApplicationModel.DataTransfer.Clipboard.SetContent(dp);
        };
        panel.Children.Add(copyErrorBtn);

        // Show/hide copy button when error changes
        ViewModel.PropertyChanged += (_, e) =>
        {
            if (e.PropertyName == nameof(ViewModel.ErrorMessage))
                copyErrorBtn.Visibility = string.IsNullOrEmpty(ViewModel.ErrorMessage)
                    ? Visibility.Collapsed : Visibility.Visible;
        };

        Grid.SetRow(panel, 5);
        return panel;
    }

    // ── Manual property change handling (replaces x:Bind) ──

    private void OnViewModelPropertyChanged(object? sender, System.ComponentModel.PropertyChangedEventArgs e)
    {
        switch (e.PropertyName)
        {
            case nameof(ViewModel.InputText):
                _inputTextBlock.Text = ViewModel.InputText ?? string.Empty;
                break;
            case nameof(ViewModel.OutputText):
                _outputTextBlock.Text = ViewModel.OutputText ?? string.Empty;
                if (!string.IsNullOrEmpty(ViewModel.OutputText))
                    DRLogger.Log($"Rewrite success: {ViewModel.OutputText?.Length ?? 0} chars", DRLogger.Category.API);
                break;
            case nameof(ViewModel.UsageInfo):
                _usageTextBlock.Text = ViewModel.UsageInfo ?? string.Empty;
                break;
            case nameof(ViewModel.ErrorMessage):
                _errorTextBlock.Text = ViewModel.ErrorMessage ?? string.Empty;
                if (!string.IsNullOrEmpty(ViewModel.ErrorMessage))
                    DRLogger.Log($"Rewrite error: {ViewModel.ErrorMessage}", DRLogger.Category.API);
                break;
            case nameof(ViewModel.IsLoading):
                _loadingOverlay.Visibility = ViewModel.IsLoading ? Visibility.Visible : Visibility.Collapsed;
                _loadingRing.IsActive = ViewModel.IsLoading;
                break;
            case nameof(ViewModel.GrammarResult):
                ApplyGrammarResult();
                break;
        }
    }

    private void ApplyGrammarResult()
    {
        // GrammarCheckView temporarily disabled — see BuildResultArea for context.
        // For now, grammar-check responses fall back to plain text via the
        // existing TextBlock path.
        if (_grammarHost == null) return;
    }

    /// <summary>
    /// Set the input text before showing.
    /// </summary>
    public void SetInputText(string text)
    {
        ViewModel.InputText = text;
        ViewModel.OutputText = string.Empty;
        ViewModel.ErrorMessage = string.Empty;
        ViewModel.UsageInfo = string.Empty;
        ViewModel.SelectedTone = null;
        UpdateToneButtonStyles(null);
    }

    /// <summary>
    /// Loads captured text into the panel and brings it to the foreground.
    /// Call from the UI thread (e.g., via DispatcherQueue.TryEnqueue).
    /// </summary>
    public void ShowForText(string text)
    {
        SetInputText(text);
        this.Activate();
    }

    private void OnToneClick(object sender, RoutedEventArgs e)
    {
        if (sender is Button btn && btn.Tag is string toneName && ToneMap.TryGetValue(toneName, out var tone))
        {
            DRLogger.Log($"Tone selected: {toneName}", DRLogger.Category.PANEL);
            UpdateToneButtonStyles(toneName);

            if (ViewModel.RewriteCommand.CanExecute(tone))
            {
                ViewModel.RewriteCommand.Execute(tone);
            }
        }
    }

    private void UpdateToneButtonStyles(string? selectedToneName)
    {
        foreach (var (name, btn) in _toneButtons)
        {
            ApplyToneButtonStyle(btn, name == selectedToneName);
        }
    }
}
