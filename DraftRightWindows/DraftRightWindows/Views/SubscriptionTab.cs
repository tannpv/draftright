using System;
using System.Drawing;
using System.Linq;
using System.Threading.Tasks;
using DraftRightWindows.Helpers;
using DraftRightWindows.Models;
using DraftRightWindows.Services;
using WinForms = System.Windows.Forms;

namespace DraftRightWindows.Views;

/// <summary>
/// UserControl rendered inside the Settings TabPage.  Mirrors the
/// Flutter SubscriptionScreen and macOS SubscriptionView: shows
/// plan + usage, lists payment methods for free users, shows the
/// Manage button for paid users.  Acts as
/// <see cref="IPaymentSheetPresenter"/> so handlers can open the
/// QR / bank dialogs without depending on Form types directly.
/// </summary>
public sealed class SubscriptionTab : WinForms.UserControl, IPaymentSheetPresenter
{
    private static readonly Color BgDark      = Color.FromArgb(15, 23, 42);
    private static readonly Color CardBg      = Color.FromArgb(30, 41, 59);
    private static readonly Color BrandBlue   = Color.FromArgb(93, 135, 255);
    private static readonly Color TextPrimary = Color.FromArgb(226, 232, 240);
    private static readonly Color TextMuted   = Color.FromArgb(148, 163, 184);
    private static readonly Color BorderColor = Color.FromArgb(51, 65, 85);

    private readonly PaymentService _payments;
    private readonly WinForms.Panel _content = new();
    private bool _isStarting;
    private PaymentMethodKind? _startingKind;

    public SubscriptionTab()
    {
        _payments = new PaymentService(App.Api);
        Dock = WinForms.DockStyle.Fill;
        BackColor = BgDark;
        ForeColor = TextPrimary;
        _content.Dock = WinForms.DockStyle.Fill;
        _content.AutoScroll = true;
        Controls.Add(_content);
        HandleCreated += async (_, _) => await RefreshAsync();
    }

    // MARK: Refresh

    private async Task RefreshAsync()
    {
        ShowMessage("Loading subscription…");
        try
        {
            var subRawTask = App.Api.GetSubscriptionAsync();
            var methodsTask = _payments.ListAvailableMethodsAsync();
            await Task.WhenAll(subRawTask, methodsTask);
            if (IsDisposed) return;
            BeginInvoke(new Action(() => Render(subRawTask.Result, methodsTask.Result)));
        }
        catch (Exception e)
        {
            if (IsDisposed) return;
            BeginInvoke(new Action(() => ShowMessage("Error: " + e.Message)));
        }
    }

    private void ShowMessage(string message)
    {
        _content.Controls.Clear();
        _content.Controls.Add(new WinForms.Label
        {
            Text = message,
            ForeColor = TextPrimary,
            Location = new Point(16, 16),
            AutoSize = true,
            Font = new Font("Segoe UI", 10),
        });
    }

    // MARK: Rendering

    private void Render(SubscriptionResponse sub, System.Collections.Generic.List<PaymentMethodKind> methods)
    {
        _content.Controls.Clear();
        int y = 12;

        AddHeader("Subscription", ref y);

        var billing = sub.Plan?.BillingPeriod ?? "none";
        var isFree = string.IsNullOrEmpty(billing) || billing == "none";

        AddRow("Plan",        sub.Plan?.Name ?? "Free",     ref y);
        AddRow("Billing",     BillingLabel(billing),         ref y);
        AddRow("Status",      StatusLabel(sub.Status),       ref y);
        AddRow("Usage today", $"{sub.UsageToday} / {sub.Plan?.DailyLimit ?? 10}", ref y);

        y += 14;

        if (isFree)
        {
            AddUpgradeSection(methods, ref y);
        }
        else
        {
            AddManageSection(ref y);
        }
    }

    private void AddHeader(string text, ref int y)
    {
        _content.Controls.Add(new WinForms.Label
        {
            Text = text,
            Font = new Font("Segoe UI", 14, FontStyle.Bold),
            ForeColor = TextPrimary,
            Location = new Point(16, y),
            AutoSize = true,
        });
        y += 36;
    }

    private void AddRow(string label, string value, ref int y)
    {
        _content.Controls.Add(new WinForms.Label
        {
            Text = label,
            Font = new Font("Segoe UI", 9),
            ForeColor = TextMuted,
            Location = new Point(16, y),
            AutoSize = true,
        });
        _content.Controls.Add(new WinForms.Label
        {
            Text = value,
            Font = new Font("Segoe UI", 10, FontStyle.Bold),
            ForeColor = TextPrimary,
            Location = new Point(140, y - 2),
            AutoSize = true,
        });
        y += 24;
    }

    private void AddUpgradeSection(System.Collections.Generic.List<PaymentMethodKind> methods, ref int y)
    {
        _content.Controls.Add(new WinForms.Label
        {
            Text = "Upgrade to Pro",
            Font = new Font("Segoe UI", 11, FontStyle.Bold),
            ForeColor = TextPrimary,
            Location = new Point(16, y),
            AutoSize = true,
        });
        y += 24;

        _content.Controls.Add(new WinForms.Label
        {
            Text = "Pick a payment method.  Your plan activates automatically once payment completes.",
            Font = new Font("Segoe UI", 9),
            ForeColor = TextMuted,
            Location = new Point(16, y),
            Size = new Size(440, 32),
        });
        y += 36;

        if (methods.Count == 0)
        {
            _content.Controls.Add(new WinForms.Label
            {
                Text = "No payment methods are enabled yet. Please check back later.",
                ForeColor = TextMuted,
                Location = new Point(16, y),
                AutoSize = true,
            });
            return;
        }

        foreach (var kind in methods)
        {
            AddMethodTile(kind, ref y);
        }
    }

    private void AddMethodTile(PaymentMethodKind kind, ref int y)
    {
        var d = PaymentMethodDescriptor.ForKind(kind);

        var tile = new WinForms.Panel
        {
            Location = new Point(16, y),
            Size = new Size(448, 64),
            BackColor = CardBg,
            Cursor = WinForms.Cursors.Hand,
        };
        tile.Paint += (s, e) =>
        {
            using var pen = new Pen(BorderColor, 1);
            e.Graphics.DrawRectangle(pen, 0, 0, tile.Width - 1, tile.Height - 1);
        };
        tile.Controls.Add(new WinForms.Label
        {
            Text = d.DisplayName,
            Font = new Font("Segoe UI", 10, FontStyle.Bold),
            ForeColor = TextPrimary,
            Location = new Point(12, 8),
            AutoSize = true,
        });
        tile.Controls.Add(new WinForms.Label
        {
            Text = d.Description,
            Font = new Font("Segoe UI", 8),
            ForeColor = TextMuted,
            Location = new Point(12, 32),
            AutoSize = true,
        });
        var chevron = new WinForms.Label
        {
            Text = "›",
            Font = new Font("Segoe UI", 18, FontStyle.Bold),
            ForeColor = TextMuted,
            Location = new Point(420, 18),
            AutoSize = true,
        };
        tile.Controls.Add(chevron);

        EventHandler handler = async (_, _) =>
        {
            if (_isStarting) return;
            _isStarting = true;
            _startingKind = kind;
            chevron.Text = "…";
            try
            {
                var planId = await _payments.ResolveProPlanIdAsync();
                await _payments.UpgradeAsync(kind, planId, this);
            }
            catch (Exception e)
            {
                WinForms.MessageBox.Show(this, e.Message, "Upgrade failed",
                    WinForms.MessageBoxButtons.OK, WinForms.MessageBoxIcon.Error);
            }
            finally
            {
                _isStarting = false;
                _startingKind = null;
                chevron.Text = "›";
            }
        };
        tile.Click += handler;
        foreach (WinForms.Control c in tile.Controls) c.Click += handler;

        _content.Controls.Add(tile);
        y += 72;
    }

    private void AddManageSection(ref int y)
    {
        var btn = new WinForms.Button
        {
            Text = "Manage subscription",
            Location = new Point(16, y),
            Size = new Size(220, 36),
            BackColor = BrandBlue,
            ForeColor = Color.White,
            FlatStyle = WinForms.FlatStyle.Flat,
            Font = new Font("Segoe UI", 9, FontStyle.Bold),
        };
        btn.Click += async (_, _) =>
        {
            btn.Enabled = false;
            btn.Text = "Opening…";
            try
            {
                await _payments.OpenCustomerPortalAsync();
            }
            catch (Exception e)
            {
                WinForms.MessageBox.Show(this, e.Message, "Could not open portal",
                    WinForms.MessageBoxButtons.OK, WinForms.MessageBoxIcon.Error);
            }
            finally
            {
                btn.Enabled = true;
                btn.Text = "Manage subscription";
            }
        };
        _content.Controls.Add(btn);
        y += 44;

        _content.Controls.Add(new WinForms.Label
        {
            Text = "Cancel, change plan, or update your payment method.",
            Font = new Font("Segoe UI", 9),
            ForeColor = TextMuted,
            Location = new Point(16, y),
            AutoSize = true,
        });
    }

    private static string BillingLabel(string p) => p switch
    {
        "none"    => "Free",
        "monthly" => "Monthly",
        "yearly"  => "Yearly",
        _         => p,
    };

    private static string StatusLabel(string s) => s switch
    {
        "active"    => "Active",
        "expired"   => "Expired",
        "cancelled" => "Cancelled",
        _           => s,
    };

    // MARK: IPaymentSheetPresenter

    public void PresentQrDialog(QrCheckout checkout, IObservable<PaymentStatusUpdate>? statusStream)
    {
        if (InvokeRequired) { BeginInvoke(new Action(() => PresentQrDialog(checkout, statusStream))); return; }
        using var dlg = new QrCheckoutDialog(checkout, statusStream);
        dlg.ShowDialog(this);
        _ = RefreshAsync();
    }

    public void PresentBankTransferDialog(BankTransferCheckout checkout, IObservable<PaymentStatusUpdate>? statusStream)
    {
        if (InvokeRequired) { BeginInvoke(new Action(() => PresentBankTransferDialog(checkout, statusStream))); return; }
        using var dlg = new BankTransferDialog(checkout, statusStream);
        dlg.ShowDialog(this);
        _ = RefreshAsync();
    }
}
