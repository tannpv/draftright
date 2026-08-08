using System;
using System.Threading;
using System.Windows;
using System.Windows.Controls;
using System.Windows.Media;
using DraftRightWindows.Services;
using Wpf.Ui.Controls;

namespace DraftRightWindows.Views;

/// <summary>
/// Throwaway WPF-UI window, the counterpart to the WinUI 3 spike that failed.
///
/// WinUI 3 could not render on unpackaged builds — COMException
/// "element not found", the resource manager unable to resolve the app's
/// resource map even after draftright.iss started regenerating resources.pri
/// (see #155). WPF has no PRI concept at all, so the question here is whether
/// it renders AND whether WPF can share a process with WindowsAppSDK, which is
/// the part nobody has tried.
///
/// Runs on its own STA thread with its own Dispatcher, exactly as
/// ReportBugDialog does. WPF and WinUI each want to own a UI thread, so
/// keeping this one separate is what makes the coexistence question
/// answerable rather than a guess.
///
/// Colours come from <see cref="Theme"/> so the spike restates no palette
/// values. The System.Drawing to WPF Media bridge below is the only piece
/// worth keeping if WPF UI is adopted.
///
/// DELETE with the spike either way — see #155.
/// </summary>
internal static class WpfUiSpikeWindow
{
    /// <summary>Opens the spike on a dedicated STA thread. Returns immediately.</summary>
    public static void Show()
    {
        var thread = new Thread(RunWindow);
        thread.SetApartmentState(ApartmentState.STA);
        thread.IsBackground = true;
        thread.Start();
    }

    private static void RunWindow()
    {
        try
        {
            var window = BuildWindow();
            window.Closed += (_, _) => System.Windows.Threading.Dispatcher
                .CurrentDispatcher.InvokeShutdown();
            window.Show();
            System.Windows.Threading.Dispatcher.Run();
        }
        catch (Exception ex)
        {
            // Same contract as the WinUI spike: report, never take the app down.
            DRLogger.Error($"WPF UI spike FAILED: {ex}", DRLogger.Category.APP);
            System.Windows.Forms.MessageBox.Show(
                $"WPF UI could not render either.\n\n{ex.GetType().Name}: {ex.Message}\n\n"
                + "Both modern toolkits are then ruled out for this process, and the "
                + "realistic options are polishing WinForms or splitting the UI out.",
                "WPF UI spike failed",
                System.Windows.Forms.MessageBoxButtons.OK,
                System.Windows.Forms.MessageBoxIcon.Error);
        }
    }

    private static Window BuildWindow()
    {
        // FluentWindow is WPF-UI's themed shell — Mica backdrop and a custom
        // title bar, the things WinForms cannot do at all.
        var window = new FluentWindow
        {
            Title = "WPF UI spike — is Fluent viable?",
            Width = 560,
            Height = 620,
            WindowBackdropType = Wpf.Ui.Controls.WindowBackdropType.Mica,
            ExtendsContentIntoTitleBar = true,
        };

        // WPF-UI ships its control templates and theme brushes as resource
        // dictionaries that a normal app merges in App.xaml. This process has
        // no App.xaml — it is a WindowsAppSDK app with no XAML at all — so
        // nothing loaded them, every control resolved to no template, and the
        // window rendered as a bare white rectangle (bug report #58).
        //
        // Merging them into the window's own resources scopes the theme to
        // this window, which is what we want anyway: the spike must not
        // restyle the WinForms UI around it.
        window.Resources.MergedDictionaries.Add(
            new Wpf.Ui.Markup.ThemesDictionary { Theme = Wpf.Ui.Appearance.ApplicationTheme.Dark });
        window.Resources.MergedDictionaries.Add(new Wpf.Ui.Markup.ControlsDictionary());

        var root = new StackPanel { Margin = new Thickness(24) };

        root.Children.Add(new TitleBar { Title = "DraftRight" });

        root.Children.Add(new System.Windows.Controls.TextBlock
        {
            Text = "WPF UI rendered",
            FontSize = 24,
            FontWeight = FontWeights.SemiBold,
            Foreground = Brush(Theme.TextPrimary),
            Margin = new Thickness(0, 12, 0, 4),
        });

        root.Children.Add(new System.Windows.Controls.TextBlock
        {
            Text = "If you can read this with a translucent background, Fluent is "
                 + "available on unpackaged builds and the WinForms panels can migrate.",
            TextWrapping = TextWrapping.Wrap,
            Foreground = Brush(Theme.TextMuted),
            Margin = new Thickness(0, 0, 0, 16),
        });

        // The same spread the WinUI spike used, so the two are comparable.
        root.Children.Add(new Wpf.Ui.Controls.TextBox
        {
            PlaceholderText = "TextBox — templated input",
            Margin = new Thickness(0, 0, 0, 12),
        });

        var combo = new System.Windows.Controls.ComboBox { Margin = new Thickness(0, 0, 0, 12) };
        foreach (var tone in new[] { "Polished", "Concise", "Technical" }) combo.Items.Add(tone);
        combo.SelectedIndex = 0;
        root.Children.Add(combo);

        root.Children.Add(new ProgressRing
        {
            IsIndeterminate = true,
            Width = 32,
            Height = 32,
            HorizontalAlignment = HorizontalAlignment.Left,
            Margin = new Thickness(0, 0, 0, 12),
        });

        var status = new System.Windows.Controls.TextBlock
        {
            Foreground = Brush(Theme.SuccessGreen),
            TextWrapping = TextWrapping.Wrap,
            Margin = new Thickness(0, 8, 0, 0),
        };

        var button = new Wpf.Ui.Controls.Button
        {
            Content = "Click me — proves input works too",
            Appearance = Wpf.Ui.Controls.ControlAppearance.Primary,
        };
        button.Click += (_, _) =>
            status.Text = $"Button clicked at {DateTime.Now:HH:mm:ss}. Interaction works.";
        root.Children.Add(button);
        root.Children.Add(status);

        window.Content = root;
        DRLogger.Log("WPF UI spike window built — content assigned without throwing",
                     DRLogger.Category.APP);
        return window;
    }

    /// <summary>
    /// System.Drawing colour as a WPF brush, so the spike reuses
    /// <see cref="Theme"/> rather than restating hex values (Rule #1).
    /// Frozen because these are shared across a UI thread boundary and never
    /// mutate — an unfrozen brush would take a lock on every render.
    /// </summary>
    private static SolidColorBrush Brush(System.Drawing.Color c)
    {
        var brush = new SolidColorBrush(Color.FromArgb(c.A, c.R, c.G, c.B));
        brush.Freeze();
        return brush;
    }
}
