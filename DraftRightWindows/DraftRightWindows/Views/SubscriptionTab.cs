using System;
using System.Collections.Generic;
using System.Threading.Tasks;
using System.Windows;
using System.Windows.Controls;
using System.Windows.Input;
using DraftRightWindows.Models;
using DraftRightWindows.Services;
using Wpf.Ui.Controls;
using WpfTextBlock = System.Windows.Controls.TextBlock;
using F = DraftRightWindows.Views.FluentFormControls;

namespace DraftRightWindows.Views;

/// <summary>
/// The Subscription tab in WPF (#159) — plan + usage, payment-method tiles for
/// free users, Manage button for paid users. Mirrors the Flutter / macOS
/// subscription surface. Acts as <see cref="IPaymentSheetPresenter"/> so the
/// payment handlers can open the checkout dialogs without depending on view
/// types. The QR and bank-transfer dialogs are WPF (#160) and open modal to the
/// Settings window.
/// </summary>
public sealed class SubscriptionTab : UserControl, IPaymentSheetPresenter
{
    private readonly PaymentService _payments;
    private readonly StackPanel _content = new() { Margin = new Thickness(F.ContentPad) };
    private bool _isStarting;
    private BillingPeriod _billingPeriod = BillingPeriod.Monthly;

    public SubscriptionTab()
    {
        _payments = new PaymentService(App.Api);
        Content = new ScrollViewer
        {
            VerticalScrollBarVisibility = ScrollBarVisibility.Auto,
            HorizontalScrollBarVisibility = ScrollBarVisibility.Disabled,
            Content = _content,
        };
        Loaded += async (_, _) => await RefreshAsync();
    }

    private async Task RefreshAsync()
    {
        ShowMessage("Loading subscription…");
        try
        {
            var subTask = App.Api.GetSubscriptionAsync();
            var methodsTask = _payments.ListAvailableMethodsAsync();
            await Task.WhenAll(subTask, methodsTask);
            Render(subTask.Result, methodsTask.Result);
        }
        catch (Exception e)
        {
            ShowMessage("Error: " + e.Message);
        }
    }

    private void ShowMessage(string message)
    {
        _content.Children.Clear();
        _content.Children.Add(new WpfTextBlock
        {
            Text = message,
            Foreground = Theme.WpfBrush(Theme.TextPrimary),
        });
    }

    private void Render(SubscriptionResponse sub, List<PaymentMethodKind> methods)
    {
        _content.Children.Clear();

        _content.Children.Add(new WpfTextBlock
        {
            Text = "Subscription",
            FontSize = 18,
            FontWeight = FontWeights.Bold,
            Foreground = Theme.WpfBrush(Theme.TextPrimary),
            Margin = new Thickness(0, 0, 0, F.SectionGap),
        });

        var billing = sub.Plan?.BillingPeriod ?? "none";
        var isFree = string.IsNullOrEmpty(billing) || billing == "none";

        AddRow("Plan", sub.Plan?.Name ?? "Free");
        AddRow("Billing", BillingLabel(billing));
        AddRow("Status", StatusLabel(sub.Status));
        AddRow("Usage today", $"{sub.UsageToday} / {sub.Plan?.DailyLimit ?? 10}");

        _content.Children.Add(new WpfTextBlock { Height = F.SectionGap });

        if (isFree) AddUpgradeSection(methods);
        else AddManageSection();
    }

    private void AddRow(string label, string value)
    {
        var grid = new Grid { Margin = new Thickness(0, 0, 0, F.LabelGap) };
        grid.ColumnDefinitions.Add(new ColumnDefinition { Width = new GridLength(130) });
        grid.ColumnDefinitions.Add(new ColumnDefinition { Width = new GridLength(1, GridUnitType.Star) });
        var l = new WpfTextBlock { Text = label, FontSize = 12, Foreground = Theme.WpfBrush(Theme.TextMuted) };
        var v = new WpfTextBlock { Text = value, FontWeight = FontWeights.SemiBold, Foreground = Theme.WpfBrush(Theme.TextPrimary) };
        Grid.SetColumn(l, 0); Grid.SetColumn(v, 1);
        grid.Children.Add(l); grid.Children.Add(v);
        _content.Children.Add(grid);
    }

    private void AddUpgradeSection(List<PaymentMethodKind> methods)
    {
        _content.Children.Add(new WpfTextBlock
        {
            Text = "Upgrade to Pro",
            FontSize = 14,
            FontWeight = FontWeights.Bold,
            Foreground = Theme.WpfBrush(Theme.TextPrimary),
            Margin = new Thickness(0, 0, 0, F.LabelGap),
        });
        _content.Children.Add(new WpfTextBlock
        {
            Text = "Pick a billing cadence, then a payment method. Your plan activates automatically once payment completes.",
            Foreground = Theme.WpfBrush(Theme.TextMuted),
            TextWrapping = TextWrapping.Wrap,
            Margin = new Thickness(0, 0, 0, F.FieldGap),
        });

        AddBillingPeriodPicker();

        if (methods.Count == 0)
        {
            _content.Children.Add(new WpfTextBlock
            {
                Text = "No payment methods are enabled yet. Please check back later.",
                Foreground = Theme.WpfBrush(Theme.TextMuted),
            });
            return;
        }

        foreach (var kind in methods) AddMethodTile(kind);
    }

    // Monthly / Yearly segmented picker — one button per BillingPeriod value.
    private void AddBillingPeriodPicker()
    {
        var row = new StackPanel { Orientation = System.Windows.Controls.Orientation.Horizontal, Margin = new Thickness(0, 0, 0, F.FieldGap) };
        foreach (var period in Enum.GetValues<BillingPeriod>())
        {
            var selected = period == _billingPeriod;
            var btn = F.Button(period.DisplayName(),
                selected ? ControlAppearance.Primary : ControlAppearance.Secondary,
                () =>
                {
                    if (_billingPeriod == period) return;
                    _billingPeriod = period;
                    _ = RefreshAsync();
                });
            btn.Margin = new Thickness(0, 0, F.ButtonGap, 0);
            row.Children.Add(btn);
        }
        _content.Children.Add(row);
    }

    private void AddMethodTile(PaymentMethodKind kind)
    {
        var d = PaymentMethodDescriptor.ForKind(kind);

        var title = new WpfTextBlock { Text = d.DisplayName, FontWeight = FontWeights.Bold, Foreground = Theme.WpfBrush(Theme.TextPrimary) };
        var desc = new WpfTextBlock { Text = d.Description, FontSize = 12, Foreground = Theme.WpfBrush(Theme.TextMuted), TextWrapping = TextWrapping.Wrap };
        var text = new StackPanel();
        text.Children.Add(title);
        text.Children.Add(desc);

        var chevron = new WpfTextBlock
        {
            Text = "›",
            FontSize = 20,
            FontWeight = FontWeights.Bold,
            Foreground = Theme.WpfBrush(Theme.TextMuted),
            VerticalAlignment = VerticalAlignment.Center,
        };

        var grid = new Grid();
        grid.ColumnDefinitions.Add(new ColumnDefinition { Width = new GridLength(1, GridUnitType.Star) });
        grid.ColumnDefinitions.Add(new ColumnDefinition { Width = GridLength.Auto });
        Grid.SetColumn(text, 0); Grid.SetColumn(chevron, 1);
        grid.Children.Add(text); grid.Children.Add(chevron);

        var tile = new Border
        {
            Background = Theme.WpfBrush(Theme.CardBg),
            BorderBrush = Theme.WpfBrush(Theme.BorderColor),
            BorderThickness = new Thickness(1),
            CornerRadius = new CornerRadius(4),
            Padding = new Thickness(12),
            Margin = new Thickness(0, 0, 0, F.ButtonGap),
            Cursor = Cursors.Hand,
            Child = grid,
        };
        tile.MouseLeftButtonUp += async (_, _) =>
        {
            if (_isStarting) return;
            _isStarting = true;
            chevron.Text = "…";
            try
            {
                var planId = await _payments.ResolveProPlanIdAsync(kind, _billingPeriod);
                await _payments.UpgradeAsync(kind, planId, this);
            }
            catch (Exception e)
            {
                System.Windows.MessageBox.Show(e.Message, "Upgrade failed", System.Windows.MessageBoxButton.OK, System.Windows.MessageBoxImage.Error);
            }
            finally
            {
                _isStarting = false;
                chevron.Text = "›";
            }
        };
        _content.Children.Add(tile);
    }

    private void AddManageSection()
    {
        var btn = F.PrimaryButton("Manage subscription", () => { });
        btn.Margin = new Thickness(0, 0, 0, F.FieldGap);
        btn.Click += async (_, _) =>
        {
            btn.IsEnabled = false;
            btn.Content = "Opening…";
            try
            {
                await _payments.OpenCustomerPortalAsync();
            }
            catch (ApiException api) when (api.StatusCode == System.Net.HttpStatusCode.NotFound)
            {
                System.Windows.MessageBox.Show(
                    "This plan has no self-service billing portal — it was granted by an "
                    + "administrator or paid via QR code / bank transfer. Please contact "
                    + "support to change or cancel it.",
                    "Manage subscription", System.Windows.MessageBoxButton.OK, System.Windows.MessageBoxImage.Information);
            }
            catch (Exception e)
            {
                System.Windows.MessageBox.Show(e.Message, "Could not open portal", System.Windows.MessageBoxButton.OK, System.Windows.MessageBoxImage.Error);
            }
            finally
            {
                btn.IsEnabled = true;
                btn.Content = "Manage subscription";
            }
        };
        _content.Children.Add(btn);
        _content.Children.Add(new WpfTextBlock
        {
            Text = "Cancel, change plan, or update your payment method.",
            Foreground = Theme.WpfBrush(Theme.TextMuted),
        });
    }

    private static string BillingLabel(string p) => p switch
    {
        "none" => "Free",
        "monthly" => "Monthly",
        "yearly" => "Yearly",
        _ => p,
    };

    private static string StatusLabel(string s) => s switch
    {
        "active" => "Active",
        "expired" => "Expired",
        "cancelled" => "Cancelled",
        _ => s,
    };

    // ── IPaymentSheetPresenter ────────────────────────────────────────────────

    public void PresentQrDialog(QrCheckout checkout, IObservable<PaymentStatusUpdate>? statusStream)
    {
        if (!Dispatcher.CheckAccess()) { Dispatcher.BeginInvoke(new Action(() => PresentQrDialog(checkout, statusStream))); return; }
        new QrCheckoutDialog(checkout, statusStream) { Owner = Window.GetWindow(this) }.ShowDialog();
        _ = RefreshAsync();
    }

    public void PresentBankTransferDialog(BankTransferCheckout checkout, IObservable<PaymentStatusUpdate>? statusStream)
    {
        if (!Dispatcher.CheckAccess()) { Dispatcher.BeginInvoke(new Action(() => PresentBankTransferDialog(checkout, statusStream))); return; }
        new BankTransferDialog(checkout, statusStream) { Owner = Window.GetWindow(this) }.ShowDialog();
        _ = RefreshAsync();
    }
}
