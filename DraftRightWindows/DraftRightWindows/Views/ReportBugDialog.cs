using System;
using System.Collections.Generic;
using System.IO;
using System.Threading;
using System.Threading.Tasks;
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

namespace DraftRightWindows.Views;

/// <summary>
/// "Report a bug" dialog — WPF UI (#158, phase 2 of the #156 migration).
///
/// — multi-line description (min 10 chars, <see cref="BugReportService.IsDescriptionValid"/>)
/// — optional email (auto-filled when signed in, editable when not)
/// — optional screenshot via four inputs: browse, drag-drop, Ctrl+V, Capture
/// — Submit posts to <see cref="BugReportService"/>; on success the dialog
///   closes and the caller shows a tray balloon.
///
/// Public API (<see cref="Show"/>) is unchanged from the WinForms version, so
/// the tray menu and Settings → Advanced callers did not change. Capture +
/// downscale live in <see cref="ScreenCapture"/>, shared with the phase-4
/// Suggest-a-feature dialog rather than copied.
/// </summary>
internal static class ReportBugDialog
{
    /// <summary>
    /// Opens the dialog on its own STA thread with its own dispatcher, so it
    /// works regardless of which thread the caller (tray, settings, hotkey…)
    /// is on. Same pattern as the rewrite panel. <paramref name="ownerHwnd"/>
    /// is accepted for API compatibility; WPF owner across threads is not set.
    /// </summary>
    public static void Show(IntPtr ownerHwnd = default)
    {
        var thread = new Thread(() =>
        {
            try
            {
                var win = new BugReportWindow();
                // Shut the dispatcher down with the window so the thread ends
                // instead of pumping an empty loop forever.
                win.Closed += (_, _) => Dispatcher.CurrentDispatcher.InvokeShutdown();
                win.Show();
                win.Activate();
                Dispatcher.Run();
            }
            catch (Exception ex)
            {
                DRLogger.Error($"ReportBugDialog: EXCEPTION {ex.GetType().Name}: {ex.Message}", DRLogger.Category.APP);
            }
        });
        thread.SetApartmentState(ApartmentState.STA);
        thread.IsBackground = true;
        thread.Start();
    }
}

/// <summary>The bug-report window. Derives from <see cref="FluentWindowBase"/>
/// so the WPF-UI theme dictionaries (#58) and the dark background (#163) are
/// inherited, not re-inlined.</summary>
internal sealed class BugReportWindow : FluentWindowBase
{
    private readonly WpfTextBox _descBox;
    private readonly WpfTextBox? _emailBox;
    private readonly Border _dropZone;
    private readonly WpfTextBlock _dropHint;
    private readonly WpfImage _preview;
    private readonly WpfTextBlock _fileLabel;
    private readonly WpfButton _clearBtn;
    private readonly WpfButton _captureBtn;
    private readonly WpfButton _submitBtn;
    private readonly WpfButton _cancelBtn;
    private readonly InfoBar _status;
    private readonly ProgressRing _progress;

    private readonly bool _isSignedIn;

    // Layout spacing — one source each, so every section lines up the same.
    private const double ContentPad = 24;   // window content padding
    private const double SectionGap = 16;    // between sections
    private const double LabelGap = 4;       // caption/label to its control
    private const double ButtonGap = 8;      // between adjacent buttons

    // The path handed to the upload. For browse/drag-file this is the user's
    // own file (never deleted). For paste/drag-bitmap/capture it is a temp PNG
    // we own — tracked separately in _tempScreenshotPath so cleanup only ever
    // deletes files we created.
    private string? _selectedScreenshotPath;
    private string? _tempScreenshotPath;

    public BugReportWindow()
    {
        Title = "Report a bug";
        Width = 600;
        Height = 680;
        WindowStartupLocation = WindowStartupLocation.CenterScreen;
        ResizeMode = ResizeMode.NoResize;
        // Standard caption (no ExtendsContentIntoTitleBar) so content starts
        // cleanly below the title bar; the window Title is the only "Report a
        // bug" label — no separate heading duplicating it.
        // No WindowBackdropType here — FluentWindowBase forces it off so the
        // dark background paints (#163). Re-enabling Mica would wash the text out.

        var icon = Helpers.AppIcon.LoadImageSource();
        if (icon != null) Icon = icon;

        try { _isSignedIn = App.Auth?.IsLoggedIn ?? false; } catch { _isSignedIn = false; }
        string? signedInEmail = null;
        try { signedInEmail = App.Auth?.CurrentEmail; } catch { /* App.Auth may be null */ }

        // Vertical stack with uniform section spacing (SectionGap). A StackPanel
        // avoids the row-index fragility of a fixed Grid and keeps every section
        // spaced the same. The window Title supplies the "Report a bug" caption,
        // so the first line here is the intro subtitle, not a second heading.
        var panel = new StackPanel { Margin = new Thickness(ContentPad) };

        panel.Children.Add(new WpfTextBlock
        {
            Text = "Tell us what went wrong. A screenshot helps a lot.",
            Foreground = Theme.WpfBrush(Theme.TextMuted),
            TextWrapping = TextWrapping.Wrap,
            Margin = new Thickness(0, 0, 0, SectionGap),
        });

        // Description
        panel.Children.Add(Caption("What happened? (min. 10 characters)"));
        _descBox = new WpfTextBox
        {
            AcceptsReturn = true,
            TextWrapping = TextWrapping.Wrap,
            VerticalScrollBarVisibility = ScrollBarVisibility.Auto,
            Height = 120,
            Margin = new Thickness(0, LabelGap, 0, SectionGap),
        };
        panel.Children.Add(_descBox);

        // Email (only when not signed in)
        if (!_isSignedIn)
        {
            panel.Children.Add(Caption("Email (optional — so we can follow up)"));
            _emailBox = new WpfTextBox { Margin = new Thickness(0, LabelGap, 0, SectionGap) };
            panel.Children.Add(_emailBox);
        }
        else
        {
            panel.Children.Add(new WpfTextBlock
            {
                Text = $"Reporting as {signedInEmail}",
                FontStyle = FontStyles.Italic,
                Foreground = Theme.WpfBrush(Theme.SuccessGreen),
                Margin = new Thickness(0, 0, 0, SectionGap),
            });
        }

        // Screenshot label
        panel.Children.Add(Caption("Attach a screenshot (optional)"));

        // Drop zone (also hosts the preview) — click to browse, drop to attach.
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
        _preview = new WpfImage
        {
            Stretch = System.Windows.Media.Stretch.Uniform,
            Visibility = Visibility.Collapsed,
        };
        var dropContent = new Grid();
        dropContent.Children.Add(_preview);
        dropContent.Children.Add(_dropHint);
        _dropZone = new Border
        {
            Background = Theme.WpfBrush(Theme.CardBg),
            BorderBrush = Theme.WpfBrush(Theme.BorderColor),
            BorderThickness = new Thickness(1),
            CornerRadius = new CornerRadius(4),
            Height = 140,
            Margin = new Thickness(0, LabelGap, 0, LabelGap),
            Cursor = Cursors.Hand,
            AllowDrop = true,
            Child = dropContent,
        };
        _dropZone.MouseLeftButtonUp += (_, _) => Browse();
        _dropZone.DragEnter += OnDragEnter;
        _dropZone.Drop += OnDrop;
        panel.Children.Add(_dropZone);

        // File label (left, elastic) + Capture + Clear (right)
        _fileLabel = new WpfTextBlock
        {
            Foreground = Theme.WpfBrush(Theme.TextMuted),
            FontSize = 12,
            TextTrimming = TextTrimming.CharacterEllipsis,
            VerticalAlignment = VerticalAlignment.Center,
        };
        _captureBtn = ActionButton("Capture screen", ControlAppearance.Secondary, CaptureScreen);
        _clearBtn = ActionButton("Clear", ControlAppearance.Secondary, ClearScreenshot);
        _clearBtn.Visibility = Visibility.Collapsed;
        _clearBtn.Margin = new Thickness(ButtonGap, 0, 0, 0);

        var fileRow = new Grid { Margin = new Thickness(0, 0, 0, SectionGap) };
        fileRow.ColumnDefinitions.Add(new ColumnDefinition { Width = new GridLength(1, GridUnitType.Star) });
        fileRow.ColumnDefinitions.Add(new ColumnDefinition { Width = GridLength.Auto });
        fileRow.ColumnDefinitions.Add(new ColumnDefinition { Width = GridLength.Auto });
        Grid.SetColumn(_fileLabel, 0);
        Grid.SetColumn(_captureBtn, 1);
        Grid.SetColumn(_clearBtn, 2);
        fileRow.Children.Add(_fileLabel);
        fileRow.Children.Add(_captureBtn);
        fileRow.Children.Add(_clearBtn);
        panel.Children.Add(fileRow);

        // Status (errors + success)
        _status = new InfoBar
        {
            IsClosable = false,
            IsOpen = false,
            Margin = new Thickness(0, 0, 0, SectionGap),
        };
        panel.Children.Add(_status);

        // Progress
        _progress = new ProgressRing
        {
            IsIndeterminate = true,
            Width = 28,
            Height = 28,
            HorizontalAlignment = HorizontalAlignment.Left,
            Visibility = Visibility.Collapsed,
            Margin = new Thickness(0, 0, 0, SectionGap),
        };
        panel.Children.Add(_progress);

        // Actions (right-aligned)
        _cancelBtn = ActionButton("Cancel", ControlAppearance.Secondary, Close);
        _submitBtn = ActionButton("Submit", ControlAppearance.Primary, () => _ = SubmitAsync());
        _submitBtn.Margin = new Thickness(ButtonGap, 0, 0, 0);
        var actions = new StackPanel
        {
            Orientation = System.Windows.Controls.Orientation.Horizontal,
            HorizontalAlignment = HorizontalAlignment.Right,
        };
        actions.Children.Add(_cancelBtn);
        actions.Children.Add(_submitBtn);
        panel.Children.Add(actions);

        Content = panel;

        // Ctrl+V at the window level → attach a clipboard image, unless the
        // description box has focus (where Ctrl+V should paste text).
        PreviewKeyDown += OnPreviewKeyDown;
        Closed += (_, _) =>
        {
            _tempScreenshotPath?.SafeDelete();
        };
    }

    // ── UI helpers ──────────────────────────────────────────────────────────

    private WpfTextBlock Caption(string text) => new()
    {
        Text = text,
        Foreground = Theme.WpfBrush(Theme.TextMuted),
        FontSize = 12,
    };

    // One button shape for the whole dialog — MinWidth + padding so labels like
    // "Capture screen" and "Submit" never clip (they did before). RULE #1: the
    // sizing lives here once, not repeated per button.
    private static WpfButton ActionButton(string text, ControlAppearance appearance, Action onClick)
    {
        var b = new WpfButton
        {
            Content = text,
            Appearance = appearance,
            MinWidth = 96,
            Padding = new Thickness(14, 6, 14, 6),
        };
        b.Click += (_, _) => onClick();
        return b;
    }

    private void SetStatus(string text, System.Drawing.Color color)
    {
        if (string.IsNullOrEmpty(text)) { _status.IsOpen = false; return; }
        _status.Severity = color == Theme.SuccessGreen ? InfoBarSeverity.Success
            : color == Theme.TextMuted ? InfoBarSeverity.Informational
            : InfoBarSeverity.Error;
        _status.Message = text;
        _status.IsOpen = true;
    }

    // ── Attach paths ────────────────────────────────────────────────────────

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
                    else SetStatus("Only PNG and JPEG images are supported.", Theme.ErrorRed);
                }
            }
            else if (e.Data.GetData(DataFormats.Bitmap) is BitmapSource bmp)
            {
                LoadScreenshotFromBitmap(bmp, "Dropped image");
            }
        }
        catch (Exception ex)
        {
            SetStatus($"Drop failed: {ex.Message}", Theme.ErrorRed);
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
                if (img != null) { LoadScreenshotFromBitmap(img, "Pasted image"); e.Handled = true; }
            }
        }
        catch (Exception ex)
        {
            SetStatus($"Paste failed: {ex.Message}", Theme.ErrorRed);
        }
    }

    private void CaptureScreen()
    {
        // Hide first, or the shot is mostly this dialog. The delay lets the
        // compositor actually remove the window before the grab.
        Hide();
        var timer = new DispatcherTimer { Interval = TimeSpan.FromMilliseconds(ScreenCapture.HideDelayMs) };
        timer.Tick += (_, _) =>
        {
            timer.Stop();
            try
            {
                var tmp = ScreenCapture.CaptureToTempPng();
                AttachTempFile(tmp, "Captured screenshot");
            }
            catch (Exception ex)
            {
                SetStatus($"Could not capture the screen: {ex.Message}", Theme.ErrorRed);
            }
            finally
            {
                Show();
                Activate();
            }
        };
        timer.Start();
    }

    // Browse / drag-file: the user's own file, used directly (never deleted).
    private void LoadScreenshotFromFile(string path)
    {
        try
        {
            if (new FileInfo(path).Length > BugReportService.MaxScreenshotBytes)
            {
                SetStatus("Screenshot exceeds 5 MB. Please attach a smaller image.", Theme.ErrorRed);
                return;
            }
            _tempScreenshotPath?.SafeDelete();
            _tempScreenshotPath = null;
            _selectedScreenshotPath = path;
            ShowPreview(LoadUnlockedBitmap(path), Path.GetFileName(path));
        }
        catch (Exception ex)
        {
            SetStatus($"Could not load image: {ex.Message}", Theme.ErrorRed);
        }
    }

    // Paste / drag-bitmap: persist the bitmap to a temp PNG we own, then attach.
    private void LoadScreenshotFromBitmap(BitmapSource bmp, string label)
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
            AttachTempFile(tmp, label);
        }
        catch (Exception ex)
        {
            SetStatus($"Could not attach image: {ex.Message}", Theme.ErrorRed);
        }
    }

    // Capture / paste land here: a temp PNG we own becomes the attachment.
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
        SetStatus("", Theme.ErrorRed);
    }

    // OnLoad reads the whole file up front so it is not left locked on disk —
    // important because capture/paste temp files are deleted on close.
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

    // ── Submit ──────────────────────────────────────────────────────────────

    private async Task SubmitAsync()
    {
        var description = _descBox.Text?.Trim() ?? "";
        if (!BugReportService.IsDescriptionValid(description))
        {
            SetStatus("Please describe the bug in at least 10 characters.", Theme.ErrorRed);
            _descBox.Focus();
            return;
        }

        var email = _emailBox?.Text?.Trim();
        string? authToken = null;
        try { authToken = App.Auth?.AccessToken; } catch { authToken = null; }

        SetBusy(true);
        SetStatus("Sending…", Theme.TextMuted);

        var ctx = new Dictionary<string, object?>
        {
            ["app_mode"] = SafeRead(() => App.Settings?.AppMode.ToString()),
            ["backend_url"] = SafeRead(() => App.Settings?.BackendUrl),
            ["signed_in"] = _isSignedIn,
        };

        try
        {
            var result = await BugReportService.SubmitAsync(
                description: description,
                screenshotPath: _selectedScreenshotPath,
                userEmail: !string.IsNullOrWhiteSpace(email) ? email : null,
                authToken: authToken,
                context: ctx);

            if (result.Success)
            {
                DRLogger.Log($"Bug report submitted (id={result.Id ?? "?"})", DRLogger.Category.APP);
                SetStatus("Thanks! Your report was submitted.", Theme.SuccessGreen);
                var t = new DispatcherTimer { Interval = TimeSpan.FromMilliseconds(900) };
                t.Tick += (_, _) => { t.Stop(); Close(); };
                t.Start();
            }
            else
            {
                SetStatus(result.ErrorMessage ?? "Submit failed.", Theme.ErrorRed);
                SetBusy(false);
            }
        }
        catch (Exception ex)
        {
            DRLogger.Log($"Bug report submit threw: {ex}", DRLogger.Category.APP);
            SetStatus(ex.Message, Theme.ErrorRed);
            SetBusy(false);
        }
    }

    private void SetBusy(bool busy)
    {
        _submitBtn.IsEnabled = !busy;
        _cancelBtn.IsEnabled = !busy;
        _descBox.IsReadOnly = busy;
        if (_emailBox != null) _emailBox.IsReadOnly = busy;
        _progress.Visibility = busy ? Visibility.Visible : Visibility.Collapsed;
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
