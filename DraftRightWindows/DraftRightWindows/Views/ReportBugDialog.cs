using System;
using System.Collections.Generic;
using System.IO;
using System.Windows;
using System.Windows.Controls;
using System.Windows.Input;
using System.Windows.Media.Imaging;
using System.Windows.Threading;
using DraftRightWindows.Services;
using Wpf.Ui.Controls;
using WpfButton = Wpf.Ui.Controls.Button;
using WpfTextBox = Wpf.Ui.Controls.TextBox;
using WpfTextBlock = System.Windows.Controls.TextBlock;
using WpfImage = System.Windows.Controls.Image;
using F = DraftRightWindows.Views.FluentFormControls;

namespace DraftRightWindows.Views;

/// <summary>
/// "Report a bug" dialog (WPF, #158; refactored onto <see cref="FeedbackDialogBase"/>
/// in #160). Description + optional email + a screenshot via four inputs
/// (browse, drag-drop, Ctrl+V, Capture) → <see cref="BugReportService"/>. The
/// shell (launcher, identity block, status, submit flow) is shared with
/// <see cref="SuggestFeatureWindow"/>; only the description + screenshot fields
/// and the submit call are bug-specific.
/// </summary>
internal static class ReportBugDialog
{
    public static void Show(IntPtr ownerHwnd = default) =>
        FeedbackDialogBase.RunOnStaThread(() => new BugReportWindow());
}

internal sealed class BugReportWindow : FeedbackDialogBase
{
    private readonly WpfTextBox _descBox;
    private readonly WpfTextBlock _dropHint;
    private readonly WpfImage _preview;
    private readonly WpfTextBlock _fileLabel;
    private readonly WpfButton _clearBtn;

    // Upload path. For browse/drag-file this is the user's own file (never
    // deleted); for paste/drag-bitmap/capture it is a temp PNG we own, tracked
    // in _tempScreenshotPath so cleanup only deletes files we created.
    private string? _selectedScreenshotPath;
    private string? _tempScreenshotPath;

    public BugReportWindow() : base("Report a bug", width: 600, height: 680)
    {
        var panel = new StackPanel { Margin = new Thickness(F.ContentPad) };

        panel.Children.Add(new WpfTextBlock
        {
            Text = "Tell us what went wrong. A screenshot helps a lot.",
            Foreground = Theme.WpfBrush(Theme.TextMuted),
            TextWrapping = TextWrapping.Wrap,
            Margin = new Thickness(0, 0, 0, F.SectionGap),
        });

        panel.Children.Add(F.FieldLabel("What happened? (min. 10 characters)"));
        _descBox = new WpfTextBox
        {
            AcceptsReturn = true,
            TextWrapping = TextWrapping.Wrap,
            VerticalScrollBarVisibility = ScrollBarVisibility.Auto,
            Height = 120,
            Margin = new Thickness(0, 0, 0, F.FieldGap),
        };
        panel.Children.Add(_descBox);

        var identity = BuildIdentityBlock("Reporting", out var readEmail);
        panel.Children.Add(identity);

        panel.Children.Add(F.FieldLabel("Attach a screenshot (optional)"));

        _dropHint = new WpfTextBlock
        {
            Text = "Drag & drop an image here, click to browse, or press Ctrl+V to paste",
            Foreground = Theme.WpfBrush(Theme.TextMuted),
            TextWrapping = TextWrapping.Wrap,
            TextAlignment = TextAlignment.Center,
            HorizontalAlignment = HorizontalAlignment.Center,
            VerticalAlignment = VerticalAlignment.Center,
            Margin = new Thickness(12),
        };
        _preview = new WpfImage { Stretch = System.Windows.Media.Stretch.Uniform, Visibility = Visibility.Collapsed };
        var dropContent = new Grid();
        dropContent.Children.Add(_preview);
        dropContent.Children.Add(_dropHint);
        var dropZone = new Border
        {
            Background = Theme.WpfBrush(Theme.CardBg),
            BorderBrush = Theme.WpfBrush(Theme.BorderColor),
            BorderThickness = new Thickness(1),
            CornerRadius = new CornerRadius(4),
            Height = 140,
            Margin = new Thickness(0, F.LabelGap, 0, F.LabelGap),
            Cursor = Cursors.Hand,
            AllowDrop = true,
            Child = dropContent,
        };
        dropZone.MouseLeftButtonUp += (_, _) => Browse();
        dropZone.DragEnter += OnDragEnter;
        dropZone.Drop += OnDrop;
        panel.Children.Add(dropZone);

        _fileLabel = new WpfTextBlock
        {
            Foreground = Theme.WpfBrush(Theme.TextMuted),
            FontSize = 12,
            TextTrimming = TextTrimming.CharacterEllipsis,
            VerticalAlignment = VerticalAlignment.Center,
        };
        var captureBtn = F.SecondaryButton("Capture screen", CaptureScreen);
        _clearBtn = F.SecondaryButton("Clear", ClearScreenshot);
        _clearBtn.Visibility = Visibility.Collapsed;
        _clearBtn.Margin = new Thickness(F.ButtonGap, 0, 0, 0);

        var fileRow = new Grid { Margin = new Thickness(0, 0, 0, F.SectionGap) };
        fileRow.ColumnDefinitions.Add(new ColumnDefinition { Width = new GridLength(1, GridUnitType.Star) });
        fileRow.ColumnDefinitions.Add(new ColumnDefinition { Width = GridLength.Auto });
        fileRow.ColumnDefinitions.Add(new ColumnDefinition { Width = GridLength.Auto });
        Grid.SetColumn(_fileLabel, 0);
        Grid.SetColumn(captureBtn, 1);
        Grid.SetColumn(_clearBtn, 2);
        fileRow.Children.Add(_fileLabel);
        fileRow.Children.Add(captureBtn);
        fileRow.Children.Add(_clearBtn);
        panel.Children.Add(fileRow);

        panel.Children.Add(StatusBar);

        var actions = BuildActions("Submit", out var submitBtn, out var cancelBtn);
        panel.Children.Add(actions);

        submitBtn.Click += async (_, _) =>
        {
            var description = _descBox.Text?.Trim() ?? "";
            if (!BugReportService.IsDescriptionValid(description))
            {
                SetStatus(InfoBarSeverity.Error, "Please describe the bug in at least 10 characters.");
                _descBox.Focus();
                return;
            }

            var email = readEmail();
            string? authToken = null;
            try { authToken = App.Auth?.AccessToken; } catch { authToken = null; }
            var ctx = new Dictionary<string, object?>
            {
                ["app_mode"] = SafeRead(() => App.Settings?.AppMode.ToString()),
                ["backend_url"] = SafeRead(() => App.Settings?.BackendUrl),
                ["signed_in"] = IsSignedIn,
            };

            await RunSubmitAsync(submitBtn, cancelBtn,
                lockFields: () => _descBox.IsReadOnly = true,
                unlockFields: () => _descBox.IsReadOnly = false,
                successMessage: "Thanks! Your report was submitted.",
                action: async () =>
                {
                    var result = await BugReportService.SubmitAsync(
                        description: description,
                        screenshotPath: _selectedScreenshotPath,
                        userEmail: !string.IsNullOrWhiteSpace(email) ? email : null,
                        authToken: authToken,
                        context: ctx);
                    if (result.Success)
                        DRLogger.Log($"Bug report submitted (id={result.Id ?? "?"})", DRLogger.Category.APP);
                    return (result.Success, result.ErrorMessage);
                });
        };

        Content = new ScrollViewer { VerticalScrollBarVisibility = ScrollBarVisibility.Auto, Content = panel };

        PreviewKeyDown += OnPreviewKeyDown;
        Closed += (_, _) => _tempScreenshotPath?.SafeDelete();
    }

    // ── Attach paths ──────────────────────────────────────────────────────────

    private void Browse()
    {
        var dlg = new Microsoft.Win32.OpenFileDialog
        {
            Title = "Choose a screenshot",
            Filter = "Images (*.png;*.jpg;*.jpeg)|*.png;*.jpg;*.jpeg",
            CheckFileExists = true,
            Multiselect = false,
        };
        if (dlg.ShowDialog(this) == true) LoadScreenshotFromFile(dlg.FileName);
    }

    private void OnDragEnter(object sender, DragEventArgs e)
    {
        e.Effects = (e.Data.GetDataPresent(DataFormats.FileDrop) || e.Data.GetDataPresent(DataFormats.Bitmap))
            ? DragDropEffects.Copy : DragDropEffects.None;
        e.Handled = true;
    }

    private void OnDrop(object sender, DragEventArgs e)
    {
        try
        {
            if (e.Data.GetDataPresent(DataFormats.FileDrop))
            {
                if (e.Data.GetData(DataFormats.FileDrop) is string[] { Length: > 0 } files)
                {
                    var ext = Path.GetExtension(files[0]).ToLowerInvariant();
                    if (ext is ".png" or ".jpg" or ".jpeg") LoadScreenshotFromFile(files[0]);
                    else SetStatus(InfoBarSeverity.Error, "Only PNG and JPEG images are supported.");
                }
            }
            else if (e.Data.GetData(DataFormats.Bitmap) is BitmapSource bmp)
            {
                LoadScreenshotFromBitmap(bmp);
            }
        }
        catch (Exception ex)
        {
            SetStatus(InfoBarSeverity.Error, $"Drop failed: {ex.Message}");
        }
    }

    private void OnPreviewKeyDown(object sender, KeyEventArgs e)
    {
        if (e.Key != Key.V || (Keyboard.Modifiers & ModifierKeys.Control) == 0) return;
        if (Keyboard.FocusedElement == _descBox) return; // let the text box paste text
        try
        {
            if (System.Windows.Clipboard.ContainsImage())
            {
                var img = System.Windows.Clipboard.GetImage();
                if (img != null) { LoadScreenshotFromBitmap(img); e.Handled = true; }
            }
        }
        catch (Exception ex)
        {
            SetStatus(InfoBarSeverity.Error, $"Paste failed: {ex.Message}");
        }
    }

    private void CaptureScreen()
    {
        Hide();
        var timer = new DispatcherTimer { Interval = TimeSpan.FromMilliseconds(ScreenCapture.HideDelayMs) };
        timer.Tick += (_, _) =>
        {
            timer.Stop();
            try { AttachTempFile(ScreenCapture.CaptureToTempPng(), "Captured screenshot"); }
            catch (Exception ex) { SetStatus(InfoBarSeverity.Error, $"Could not capture the screen: {ex.Message}"); }
            finally { Show(); Activate(); }
        };
        timer.Start();
    }

    private void LoadScreenshotFromFile(string path)
    {
        try
        {
            if (new FileInfo(path).Length > BugReportService.MaxScreenshotBytes)
            {
                SetStatus(InfoBarSeverity.Error, "Screenshot exceeds 5 MB. Please attach a smaller image.");
                return;
            }
            _tempScreenshotPath?.SafeDelete();
            _tempScreenshotPath = null;
            _selectedScreenshotPath = path;
            ShowPreview(LoadUnlockedBitmap(path), Path.GetFileName(path));
        }
        catch (Exception ex)
        {
            SetStatus(InfoBarSeverity.Error, $"Could not load image: {ex.Message}");
        }
    }

    private void LoadScreenshotFromBitmap(BitmapSource bmp)
    {
        try
        {
            var tmp = Path.Combine(Path.GetTempPath(), $"draftright-bugreport-{Guid.NewGuid():N}.png");
            using (var fs = File.Create(tmp))
            {
                var enc = new PngBitmapEncoder();
                enc.Frames.Add(BitmapFrame.Create(bmp));
                enc.Save(fs);
            }
            AttachTempFile(tmp, "Pasted image");
        }
        catch (Exception ex)
        {
            SetStatus(InfoBarSeverity.Error, $"Could not attach image: {ex.Message}");
        }
    }

    private void AttachTempFile(string tmpPath, string label)
    {
        _tempScreenshotPath?.SafeDelete();
        _tempScreenshotPath = tmpPath;
        _selectedScreenshotPath = tmpPath;
        ShowPreview(LoadUnlockedBitmap(tmpPath), label);
    }

    private void ShowPreview(BitmapImage img, string label)
    {
        _preview.Source = img;
        _preview.Visibility = Visibility.Visible;
        _dropHint.Visibility = Visibility.Collapsed;
        _fileLabel.Text = label;
        _clearBtn.Visibility = Visibility.Visible;
        SetStatus(InfoBarSeverity.Error, "");
    }

    // OnLoad reads the whole file up front so temp files aren't left locked.
    private static BitmapImage LoadUnlockedBitmap(string path)
    {
        var bmp = new BitmapImage();
        bmp.BeginInit();
        bmp.CacheOption = BitmapCacheOption.OnLoad;
        bmp.UriSource = new Uri(path);
        bmp.EndInit();
        bmp.Freeze();
        return bmp;
    }

    private void ClearScreenshot()
    {
        _selectedScreenshotPath = null;
        _tempScreenshotPath?.SafeDelete();
        _tempScreenshotPath = null;
        _preview.Source = null;
        _preview.Visibility = Visibility.Collapsed;
        _dropHint.Visibility = Visibility.Visible;
        _fileLabel.Text = "";
        _clearBtn.Visibility = Visibility.Collapsed;
    }

    private static string? SafeRead(Func<string?> reader)
    {
        try { return reader(); } catch { return null; }
    }
}

internal static class TempPathExtensions
{
    public static void SafeDelete(this string? path)
    {
        try
        {
            if (!string.IsNullOrEmpty(path) && File.Exists(path))
                File.Delete(path);
        }
        catch
        {
            // Best-effort
        }
    }
}
