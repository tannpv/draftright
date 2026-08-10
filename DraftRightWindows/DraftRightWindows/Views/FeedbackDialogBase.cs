using System;
using System.Threading;
using System.Threading.Tasks;
using System.Windows;
using System.Windows.Controls;
using System.Windows.Threading;
using DraftRightWindows.Services;
using Wpf.Ui.Controls;
using WpfButton = Wpf.Ui.Controls.Button;
using WpfTextBox = Wpf.Ui.Controls.TextBox;
using WpfTextBlock = System.Windows.Controls.TextBlock;
using F = DraftRightWindows.Views.FluentFormControls;

namespace DraftRightWindows.Views;

/// <summary>
/// Shared shell for the feedback dialogs (#160) — Report a Bug and Suggest a
/// Feature. They mirror each other in everything but the middle fields: the
/// STA-thread launcher, the email-or-"as {email}" identity block, the status
/// line, the Cancel/Submit row, and the submit → lock → success-flash-close →
/// unlock-on-error flow all live here once. RULE #1: two mirrors, one base.
/// Subclasses supply their body fields and the async submit action.
/// </summary>
internal abstract class FeedbackDialogBase : FluentWindowBase
{
    protected bool IsSignedIn { get; }
    private readonly string? _signedInEmail;

    private readonly InfoBar _status = new()
    {
        IsClosable = false,
        IsOpen = false,
        Margin = new Thickness(0, 0, 0, F.FieldGap),
    };

    protected FeedbackDialogBase(string title, double width = 600, double height = 640)
    {
        Title = title;
        Width = width;
        Height = height;
        ResizeMode = ResizeMode.NoResize;
        WindowStartupLocation = WindowStartupLocation.CenterScreen;

        var icon = Helpers.AppIcon.LoadImageSource();
        if (icon != null) Icon = icon;

        try { IsSignedIn = App.Auth?.IsLoggedIn ?? false; } catch { IsSignedIn = false; }
        try { _signedInEmail = App.Auth?.CurrentEmail; } catch { _signedInEmail = null; }
    }

    /// <summary>Runs <paramref name="factory"/>'s window on its own STA thread
    /// with a WPF dispatcher — the launcher every feedback dialog shares.</summary>
    internal static void RunOnStaThread(Func<FeedbackDialogBase> factory) =>
        StaWindowHost.Run("feedback-dialog", factory);

    /// <summary>The status InfoBar — add it to the layout where the subclass wants it.</summary>
    protected InfoBar StatusBar => _status;

    protected void SetStatus(InfoBarSeverity severity, string? message)
    {
        if (string.IsNullOrEmpty(message)) { _status.IsOpen = false; return; }
        _status.Severity = severity;
        _status.Message = message;
        _status.IsOpen = true;
    }

    /// <summary>
    /// Builds the identity block: an email field when signed out, or a
    /// "{verb} as {email}" line when signed in. Sets <paramref name="readEmail"/>
    /// to read the entered address (null when signed in).
    /// </summary>
    protected UIElement BuildIdentityBlock(string signedInVerb, out Func<string?> readEmail)
    {
        if (!IsSignedIn)
        {
            var box = F.TextField();
            readEmail = () => box.Text?.Trim();
            var sp = new StackPanel();
            sp.Children.Add(F.FieldLabel("Email (optional — so we can follow up)"));
            sp.Children.Add(box);
            return sp;
        }

        readEmail = () => null;
        return new WpfTextBlock
        {
            Text = $"{signedInVerb} as {_signedInEmail}",
            FontStyle = FontStyles.Italic,
            Foreground = Theme.WpfBrush(Theme.SuccessGreen),
            Margin = new Thickness(0, 0, 0, F.FieldGap),
        };
    }

    /// <summary>Right-aligned Cancel + Submit row. Cancel closes the window.</summary>
    protected FrameworkElement BuildActions(string submitText, out WpfButton submit, out WpfButton cancel)
    {
        cancel = F.SecondaryButton("Cancel", Close);
        submit = F.PrimaryButton(submitText, () => { });
        submit.Margin = new Thickness(F.ButtonGap, 0, 0, 0);
        var row = new StackPanel
        {
            Orientation = System.Windows.Controls.Orientation.Horizontal,
            HorizontalAlignment = HorizontalAlignment.Right,
        };
        row.Children.Add(cancel);
        row.Children.Add(submit);
        return row;
    }

    /// <summary>
    /// The shared submit flow: disable Submit/Cancel, run <paramref name="lockFields"/>,
    /// show "Sending…", await <paramref name="action"/>. On success show
    /// <paramref name="successMessage"/> then close after a brief flash; on failure
    /// show the error and re-enable via <paramref name="unlockFields"/>.
    /// </summary>
    protected async Task RunSubmitAsync(
        WpfButton submit, WpfButton cancel,
        Action lockFields, Action unlockFields,
        string successMessage,
        Func<Task<(bool ok, string? error)>> action)
    {
        submit.IsEnabled = false;
        cancel.IsEnabled = false;
        lockFields();
        SetStatus(InfoBarSeverity.Informational, "Sending…");

        void ReEnable()
        {
            submit.IsEnabled = true;
            cancel.IsEnabled = true;
            unlockFields();
        }

        try
        {
            var (ok, error) = await action();
            if (ok)
            {
                SetStatus(InfoBarSeverity.Success, successMessage);
                var t = new DispatcherTimer { Interval = TimeSpan.FromMilliseconds(900) };
                t.Tick += (_, _) => { t.Stop(); Close(); };
                t.Start();
            }
            else
            {
                SetStatus(InfoBarSeverity.Error, error ?? "Submit failed.");
                ReEnable();
            }
        }
        catch (Exception ex)
        {
            SetStatus(InfoBarSeverity.Error, ex.Message);
            ReEnable();
        }
    }
}
