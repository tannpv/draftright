using System;
using System.Collections.Generic;
using System.Drawing;
using System.IO;
using System.Threading;
using System.Threading.Tasks;
using DraftRightWindows.Services;
using WinForms = System.Windows.Forms;

namespace DraftRightWindows.Views;

/// <summary>
/// "Report a bug" dialog. Modeled on the macOS / admin-portal flow:
/// — multi-line description (min 10 chars)
/// — optional email (auto-filled when signed in, editable when not)
/// — optional screenshot via three input methods (browse, drag-drop, paste)
/// — Submit button posts to <see cref="BugReportService"/>; on success the
///   dialog closes and the caller shows a tray balloon.
///
/// Implemented in WinForms for consistency with the rest of this app
/// (SettingsWindow, RewritePanelForm, CopyableErrorDialog) — the WinUI XAML
/// surface intentionally remains minimal because of the unpackaged-build
/// XAML resource issue documented in App.cs.
/// </summary>
internal static class ReportBugDialog
{
    // Theme — matches SettingsFormBuilder

    /// <summary>
    /// Largest edge a captured screenshot keeps, in pixels. A full-screen PNG
    /// off a 4K display runs well past the 5 MB upload cap and would only fail
    /// at submit time, so captures are downscaled first. Matches the macOS
    /// sheet, which uses the same bound for the same reason.
    /// </summary>
    private const int CaptureMaxDimension = 2000;

    /// <summary>
    /// Pause between hiding the dialog and grabbing the screen, so the
    /// compositor has actually removed our own window from the frame.
    /// macOS uses the same 250 ms.
    /// </summary>
    private const int CaptureHideDelayMs = 250;

    /// <summary>
    /// Opens the dialog on its own STA thread so it works regardless of which
    /// thread the caller (tray menu, settings, hotkey…) is on.
    /// </summary>
    public static void Show(IntPtr ownerHwnd = default)
    {
        var thread = new Thread(() => RunDialog(ownerHwnd));
        thread.SetApartmentState(ApartmentState.STA);
        thread.IsBackground = true;
        thread.Start();
    }

    private static void RunDialog(IntPtr ownerHwnd)
    {
        WinForms.Application.EnableVisualStyles();

        var form = new WinForms.Form
        {
            Text = "Report a bug",
            Width = 560,
            Height = 640,
            StartPosition = WinForms.FormStartPosition.CenterScreen,
            BackColor = Theme.BgDark,
            ForeColor = Theme.TextPrimary,
            FormBorderStyle = WinForms.FormBorderStyle.FixedSingle,
            MaximizeBox = false,
            MinimizeBox = false,
            ShowInTaskbar = true,
            KeyPreview = true, // so Ctrl+V hits the form-level handler regardless of focus
        };

        // Embedded-resource load — survives single-file publish where the .ico
        // isn't next to the exe (#78). See Helpers/AppIcon.
        var ico = Helpers.AppIcon.Load();
        if (ico != null) form.Icon = ico;

        // ── Layout ───────────────────────────────────────────
        int y = 16;

        var title = new WinForms.Label
        {
            Text = "Report a bug",
            Font = new Font("Segoe UI", 14, FontStyle.Bold),
            ForeColor = Theme.TextPrimary,
            Location = new Point(20, y),
            AutoSize = true,
        };
        form.Controls.Add(title);
        y += 32;

        var subtitle = new WinForms.Label
        {
            Text = "Tell us what went wrong. A screenshot helps a lot.",
            Font = new Font("Segoe UI", 9),
            ForeColor = Theme.TextMuted,
            Location = new Point(20, y),
            AutoSize = true,
        };
        form.Controls.Add(subtitle);
        y += 28;

        // Description
        form.Controls.Add(new WinForms.Label
        {
            Text = "What happened? (min. 10 characters)",
            Font = new Font("Segoe UI", 9),
            ForeColor = Theme.TextMuted,
            Location = new Point(20, y),
            AutoSize = true,
        });
        y += 18;

        var descBox = new WinForms.TextBox
        {
            Multiline = true,
            AcceptsReturn = true,
            ScrollBars = WinForms.ScrollBars.Vertical,
            Location = new Point(20, y),
            Size = new Size(500, 120),
            BackColor = Theme.CardBg,
            ForeColor = Theme.TextPrimary,
            BorderStyle = WinForms.BorderStyle.FixedSingle,
            Font = new Font("Segoe UI", 10),
        };
        form.Controls.Add(descBox);
        y += 132;

        // Email (only shown when not signed in)
        bool isSignedIn = false;
        string? signedInEmail = null;
        try
        {
            isSignedIn = App.Auth?.IsLoggedIn ?? false;
            signedInEmail = App.Auth?.CurrentEmail;
        }
        catch
        {
            // App.Auth may be null in test/standalone scenarios.
        }

        WinForms.TextBox? emailBox = null;
        if (!isSignedIn)
        {
            form.Controls.Add(new WinForms.Label
            {
                Text = "Email (optional — so we can follow up)",
                Font = new Font("Segoe UI", 9),
                ForeColor = Theme.TextMuted,
                Location = new Point(20, y),
                AutoSize = true,
            });
            y += 18;

            emailBox = new WinForms.TextBox
            {
                Location = new Point(20, y),
                Size = new Size(500, 30),
                BackColor = Theme.CardBg,
                ForeColor = Theme.TextPrimary,
                BorderStyle = WinForms.BorderStyle.FixedSingle,
                Font = new Font("Segoe UI", 10),
            };
            form.Controls.Add(emailBox);
            y += 44;
        }
        else
        {
            form.Controls.Add(new WinForms.Label
            {
                Text = $"Reporting as {signedInEmail}",
                Font = new Font("Segoe UI", 9, FontStyle.Italic),
                ForeColor = Theme.SuccessGreen,
                Location = new Point(20, y),
                AutoSize = true,
            });
            y += 28;
        }

        // Screenshot section
        form.Controls.Add(new WinForms.Label
        {
            Text = "Attach a screenshot (optional)",
            Font = new Font("Segoe UI", 9),
            ForeColor = Theme.TextMuted,
            Location = new Point(20, y),
            AutoSize = true,
        });
        y += 18;

        // Drop zone — also shows a thumbnail preview when an image is loaded.
        var dropZone = new WinForms.Panel
        {
            Location = new Point(20, y),
            Size = new Size(500, 120),
            BackColor = Theme.CardBg,
            BorderStyle = WinForms.BorderStyle.FixedSingle,
            AllowDrop = true,
        };
        form.Controls.Add(dropZone);

        var dropHint = new WinForms.Label
        {
            Text = "Drag & drop an image here, click to browse, or press Ctrl+V to paste",
            Font = new Font("Segoe UI", 9),
            ForeColor = Theme.TextMuted,
            Dock = WinForms.DockStyle.Fill,
            TextAlign = ContentAlignment.MiddleCenter,
            Cursor = WinForms.Cursors.Hand,
        };
        dropZone.Controls.Add(dropHint);

        var preview = new WinForms.PictureBox
        {
            Dock = WinForms.DockStyle.Fill,
            SizeMode = WinForms.PictureBoxSizeMode.Zoom,
            BackColor = Theme.CardBg,
            Visible = false,
            Cursor = WinForms.Cursors.Hand,
        };
        dropZone.Controls.Add(preview);

        y += 132;

        // Selected file label + clear button
        string? selectedScreenshotPath = null;
        var fileLabel = new WinForms.Label
        {
            Text = "",
            Font = new Font("Segoe UI", 8.5f),
            ForeColor = Theme.TextMuted,
            Location = new Point(20, y),
            Size = new Size(420, 18),
            AutoEllipsis = true,
        };
        form.Controls.Add(fileLabel);

        // Capture the screen directly — the fourth input method, and the one
        // that matters when a user is mid-bug: alt-tabbing to a snipping tool
        // often disturbs the very state they are trying to show us (#138).
        var captureBtn = new WinForms.Button
        {
            Text = "Capture screen",
            Location = new Point(330, y - 4),
            Size = new Size(120, 24),
            BackColor = Theme.CardBg,
            ForeColor = Theme.TextPrimary,
            FlatStyle = WinForms.FlatStyle.Flat,
            Font = new Font("Segoe UI", 8.5f),
            Cursor = WinForms.Cursors.Hand,
        };
        captureBtn.FlatAppearance.BorderColor = Theme.BorderColor;
        form.Controls.Add(captureBtn);

        var clearBtn = new WinForms.Button
        {
            Text = "Clear",
            Location = new Point(456, y - 4),
            Size = new Size(64, 24),
            BackColor = Theme.CardBg,
            ForeColor = Theme.TextMuted,
            FlatStyle = WinForms.FlatStyle.Flat,
            Font = new Font("Segoe UI", 8.5f),
            Visible = false,
        };
        clearBtn.FlatAppearance.BorderColor = Theme.BorderColor;
        form.Controls.Add(clearBtn);
        y += 26;

        // Status / error message
        var statusLabel = new WinForms.Label
        {
            Text = "",
            Font = new Font("Segoe UI", 9),
            ForeColor = Theme.ErrorRed,
            Location = new Point(20, y),
            Size = new Size(500, 36),
            AutoEllipsis = true,
            Visible = false,
        };
        form.Controls.Add(statusLabel);
        y += 44;

        // Progress
        var progress = new WinForms.ProgressBar
        {
            Style = WinForms.ProgressBarStyle.Marquee,
            MarqueeAnimationSpeed = 30,
            Location = new Point(20, y),
            Size = new Size(500, 6),
            Visible = false,
        };
        form.Controls.Add(progress);
        y += 14;

        // Buttons
        var cancelBtn = new WinForms.Button
        {
            Text = "Cancel",
            Location = new Point(360, y),
            Size = new Size(80, 32),
            BackColor = Theme.CardBg,
            ForeColor = Theme.TextPrimary,
            FlatStyle = WinForms.FlatStyle.Flat,
            Font = new Font("Segoe UI", 9.5f),
        };
        cancelBtn.FlatAppearance.BorderColor = Theme.BorderColor;
        form.Controls.Add(cancelBtn);

        var submitBtn = new WinForms.Button
        {
            Text = "Submit",
            Location = new Point(448, y),
            Size = new Size(72, 32),
            BackColor = Theme.BrandBlue,
            ForeColor = Color.White,
            FlatStyle = WinForms.FlatStyle.Flat,
            Font = new Font("Segoe UI", 9.5f, FontStyle.Bold),
        };
        submitBtn.FlatAppearance.BorderSize = 0;
        form.Controls.Add(submitBtn);

        // ── Helpers ─────────────────────────────────────────

        // Tracks an in-process temp file so we clean it up on close.
        string? tempScreenshotPath = null;

        void SetStatus(string text, Color color)
        {
            statusLabel.Text = text;
            statusLabel.ForeColor = color;
            statusLabel.Visible = !string.IsNullOrEmpty(text);
        }

        void LoadScreenshotFromFile(string path)
        {
            try
            {
                var info = new FileInfo(path);
                if (info.Length > Services.BugReportService.MaxScreenshotBytes)
                {
                    SetStatus("Screenshot exceeds 5 MB. Please attach a smaller image.", Theme.ErrorRed);
                    return;
                }

                // Load the image fully into memory then dispose the file handle so
                // PictureBox doesn't keep the underlying file locked.
                using (var fs = File.OpenRead(path))
                {
                    var img = Image.FromStream(fs);
                    preview.Image?.Dispose();
                    preview.Image = img;
                }

                selectedScreenshotPath = path;
                preview.Visible = true;
                dropHint.Visible = false;
                fileLabel.Text = Path.GetFileName(path);
                clearBtn.Visible = true;
                SetStatus("", Theme.ErrorRed);
            }
            catch (Exception ex)
            {
                SetStatus($"Could not load image: {ex.Message}", Theme.ErrorRed);
            }
        }

        void LoadScreenshotFromImage(Image img, string label = "Pasted image")
        {
            try
            {
                // Persist to a temp PNG so the multipart upload can stream from disk.
                tempScreenshotPath?.SafeDelete();
                var tmp = Path.Combine(Path.GetTempPath(),
                    $"draftright-bugreport-{Guid.NewGuid():N}.png");
                img.Save(tmp, System.Drawing.Imaging.ImageFormat.Png);
                tempScreenshotPath = tmp;

                preview.Image?.Dispose();
                preview.Image = (Image)img.Clone();
                preview.Visible = true;
                dropHint.Visible = false;

                selectedScreenshotPath = tmp;
                fileLabel.Text = label;
                clearBtn.Visible = true;
                SetStatus("", Theme.ErrorRed);
            }
            catch (Exception ex)
            {
                SetStatus($"Could not attach pasted image: {ex.Message}", Theme.ErrorRed);
            }
        }

        void ClearScreenshot()
        {
            selectedScreenshotPath = null;
            preview.Image?.Dispose();
            preview.Image = null;
            preview.Visible = false;
            dropHint.Visible = true;
            fileLabel.Text = "";
            clearBtn.Visible = false;
            tempScreenshotPath?.SafeDelete();
            tempScreenshotPath = null;
        }

        // ── Wiring ──────────────────────────────────────────

        // Browse via single-click on the drop zone (and the hint label).
        WinForms.MouseEventHandler browseHandler = (_, _) =>
        {
            using var dlg = new WinForms.OpenFileDialog
            {
                Title = "Choose a screenshot",
                Filter = "Images (*.png;*.jpg;*.jpeg)|*.png;*.jpg;*.jpeg",
                CheckFileExists = true,
                Multiselect = false,
            };
            if (dlg.ShowDialog(form) == WinForms.DialogResult.OK)
            {
                LoadScreenshotFromFile(dlg.FileName);
            }
        };
        dropHint.MouseClick += browseHandler;
        preview.MouseClick += browseHandler;
        dropZone.MouseClick += browseHandler;

        // Drag & drop
        dropZone.DragEnter += (_, e) =>
        {
            if (e.Data == null) return;
            if (e.Data.GetDataPresent(WinForms.DataFormats.FileDrop) ||
                e.Data.GetDataPresent(WinForms.DataFormats.Bitmap))
            {
                e.Effect = WinForms.DragDropEffects.Copy;
            }
        };
        dropZone.DragDrop += (_, e) =>
        {
            try
            {
                if (e.Data == null) return;
                if (e.Data.GetDataPresent(WinForms.DataFormats.FileDrop))
                {
                    var files = (string[]?)e.Data.GetData(WinForms.DataFormats.FileDrop);
                    if (files != null && files.Length > 0)
                    {
                        var ext = Path.GetExtension(files[0]).ToLowerInvariant();
                        if (ext is ".png" or ".jpg" or ".jpeg")
                            LoadScreenshotFromFile(files[0]);
                        else
                            SetStatus("Only PNG and JPEG images are supported.", Theme.ErrorRed);
                    }
                }
                else if (e.Data.GetDataPresent(WinForms.DataFormats.Bitmap))
                {
                    if (e.Data.GetData(WinForms.DataFormats.Bitmap) is Image img)
                        LoadScreenshotFromImage(img);
                }
            }
            catch (Exception ex)
            {
                SetStatus($"Drop failed: {ex.Message}", Theme.ErrorRed);
            }
        };

        // Form-level Ctrl+V → grab bitmap from clipboard if any.
        form.KeyDown += (_, e) =>
        {
            if (e.Control && e.KeyCode == WinForms.Keys.V)
            {
                // Description box also accepts Ctrl+V for text — only intercept
                // when the focused control is NOT the description text box.
                if (form.ActiveControl == descBox) return;

                try
                {
                    if (WinForms.Clipboard.ContainsImage())
                    {
                        var img = WinForms.Clipboard.GetImage();
                        if (img != null) LoadScreenshotFromImage(img);
                        e.Handled = true;
                    }
                }
                catch (Exception ex)
                {
                    SetStatus($"Paste failed: {ex.Message}", Theme.ErrorRed);
                }
            }
        };

        clearBtn.Click += (_, _) => ClearScreenshot();

        captureBtn.Click += (_, _) =>
        {
            // Hide ourselves first, or the screenshot is mostly this dialog.
            // The delay lets the compositor actually remove the window before
            // the grab; without it the frame still contains a ghost of it.
            form.Visible = false;
            var timer = new WinForms.Timer { Interval = CaptureHideDelayMs };
            timer.Tick += (_, _) =>
            {
                timer.Stop();
                timer.Dispose();
                try
                {
                    using var shot = CaptureScreen();
                    using var scaled = Downscale(shot, CaptureMaxDimension);
                    // Reuse the paste path: it already writes the temp PNG the
                    // multipart upload streams from, refreshes the preview, and
                    // wires up Clear.
                    LoadScreenshotFromImage(scaled, "Captured screenshot");
                }
                catch (Exception ex)
                {
                    SetStatus($"Could not capture the screen: {ex.Message}", Theme.ErrorRed);
                }
                finally
                {
                    form.Visible = true;
                    form.Activate();
                }
            };
            timer.Start();
        };
        cancelBtn.Click += (_, _) => form.Close();

        // Submit
        submitBtn.Click += async (_, _) =>
        {
            var description = descBox.Text?.Trim() ?? "";
            if (!BugReportService.IsDescriptionValid(description))
            {
                SetStatus("Please describe the bug in at least 10 characters.", Theme.ErrorRed);
                descBox.Focus();
                return;
            }

            var email = emailBox?.Text?.Trim();
            string? authToken = null;
            try { authToken = App.Auth?.AccessToken; } catch { authToken = null; }

            // Lock UI
            submitBtn.Enabled = false;
            cancelBtn.Enabled = false;
            descBox.ReadOnly = true;
            if (emailBox != null) emailBox.ReadOnly = true;
            progress.Visible = true;
            SetStatus("Sending…", Theme.TextMuted);

            var ctx = new Dictionary<string, object?>
            {
                ["app_mode"] = SafeRead(() => App.Settings?.AppMode.ToString()),
                ["backend_url"] = SafeRead(() => App.Settings?.BackendUrl),
                ["signed_in"] = isSignedIn,
            };

            try
            {
                var result = await BugReportService.SubmitAsync(
                    description: description,
                    screenshotPath: selectedScreenshotPath,
                    userEmail: !string.IsNullOrWhiteSpace(email) ? email : null,
                    authToken: authToken,
                    context: ctx);

                if (result.Success)
                {
                    DRLogger.Log($"Bug report submitted (id={result.Id ?? "?"})", DRLogger.Category.APP);
                    SetStatus("Thanks! Your report was submitted.", Theme.SuccessGreen);
                    // Brief flash of success, then close.
                    var t = new WinForms.Timer { Interval = 900 };
                    t.Tick += (_, _) =>
                    {
                        t.Stop();
                        t.Dispose();
                        form.Close();
                    };
                    t.Start();
                }
                else
                {
                    SetStatus(result.ErrorMessage ?? "Submit failed.", Theme.ErrorRed);
                    submitBtn.Enabled = true;
                    cancelBtn.Enabled = true;
                    descBox.ReadOnly = false;
                    if (emailBox != null) emailBox.ReadOnly = false;
                    progress.Visible = false;
                }
            }
            catch (Exception ex)
            {
                DRLogger.Log($"Bug report submit threw: {ex}", DRLogger.Category.APP);
                SetStatus(ex.Message, Theme.ErrorRed);
                submitBtn.Enabled = true;
                cancelBtn.Enabled = true;
                descBox.ReadOnly = false;
                if (emailBox != null) emailBox.ReadOnly = false;
                progress.Visible = false;
            }
        };

        // Cleanup on close
        form.FormClosed += (_, _) =>
        {
            preview.Image?.Dispose();
            tempScreenshotPath?.SafeDelete();
        };

        form.AcceptButton = submitBtn;
        form.CancelButton = cancelBtn;

        WinForms.Application.Run(form);
    }

    private static string? SafeRead(Func<string?> reader)
    {
        try { return reader(); } catch { return null; }
    }

    /// <summary>
    /// Grabs the primary display. Matches macOS, which captures the main
    /// display rather than every monitor — a multi-monitor grab produces an
    /// enormous, mostly-empty image that is harder to read, not easier.
    /// </summary>
    private static Bitmap CaptureScreen()
    {
        var bounds = WinForms.Screen.PrimaryScreen?.Bounds
            ?? throw new InvalidOperationException("No primary display found.");
        var bmp = new Bitmap(bounds.Width, bounds.Height);
        try
        {
            using var g = Graphics.FromImage(bmp);
            g.CopyFromScreen(bounds.Location, Point.Empty, bounds.Size);
            return bmp;
        }
        catch
        {
            bmp.Dispose();
            throw;
        }
    }

    /// <summary>
    /// Shrinks <paramref name="src"/> so its longest edge is at most
    /// <paramref name="maxDimension"/>, preserving aspect ratio. Returns a
    /// copy either way, so the caller can dispose both without special cases.
    ///
    /// A 4K screenshot saves to a PNG several times the upload cap; without
    /// this the attach would succeed and the submit would fail, which reads as
    /// the report being lost.
    /// </summary>
    private static Bitmap Downscale(Image src, int maxDimension)
    {
        var scale = Math.Min(1.0, (double)maxDimension / Math.Max(src.Width, src.Height));
        var w = Math.Max(1, (int)Math.Round(src.Width * scale));
        var h = Math.Max(1, (int)Math.Round(src.Height * scale));

        var outBmp = new Bitmap(w, h);
        try
        {
            using var g = Graphics.FromImage(outBmp);
            g.InterpolationMode = System.Drawing.Drawing2D.InterpolationMode.HighQualityBicubic;
            g.DrawImage(src, 0, 0, w, h);
            return outBmp;
        }
        catch
        {
            outBmp.Dispose();
            throw;
        }
    }
}

internal static class TempPathExtensions
{
    public static void SafeDelete(this string path)
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
