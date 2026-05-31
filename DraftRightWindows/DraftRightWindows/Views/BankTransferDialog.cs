using System;
using System.Drawing;
using DraftRightWindows.Models;
using WinForms = System.Windows.Forms;

namespace DraftRightWindows.Views;

/// <summary>
/// Modal dialog shown for `bank_transfer` checkout.  Renders the
/// account fields plus a copyable reference code.  Auto-dismisses
/// when the foreground poller reports success.
/// </summary>
public sealed class BankTransferDialog : WinForms.Form
{
    private readonly BankTransferCheckout _checkout;
    private readonly IObservable<PaymentStatusUpdate>? _statusStream;

    public BankTransferDialog(BankTransferCheckout checkout, IObservable<PaymentStatusUpdate>? statusStream)
    {
        _checkout = checkout;
        _statusStream = statusStream;
        InitializeUI();
    }

    private void InitializeUI()
    {
        Text = "Bank transfer";
        Width = 460;
        Height = 380;
        StartPosition = WinForms.FormStartPosition.CenterParent;
        BackColor = Color.FromArgb(15, 23, 42);
        ForeColor = Color.White;
        FormBorderStyle = WinForms.FormBorderStyle.FixedDialog;
        MaximizeBox = false;

        var banner = new PaymentStatusBanner(_statusStream, onConfirmed: Close);
        Controls.Add(banner);

        var heading = new WinForms.Label
        {
            Text = "Bank transfer",
            Font = new Font("Segoe UI", 14, FontStyle.Bold),
            ForeColor = Color.White,
            AutoSize = true,
            Location = new Point(16, 56),
        };
        var blurb = new WinForms.Label
        {
            Text = "Transfer this exact amount from any Vietnamese bank.\n" +
                   "The reference code links the payment to your account;\n" +
                   "your plan activates automatically once received.",
            ForeColor = Color.LightGray,
            AutoSize = true,
            Location = new Point(16, 96),
        };
        Controls.Add(heading);
        Controls.Add(blurb);

        int y = 170;
        BankInfoTable.Render(this, _checkout.Info, ref y);

        var close = new WinForms.Button
        {
            Text = "Close",
            Location = new Point(360, Height - 80),
            Size = new Size(80, 32),
            BackColor = Color.FromArgb(51, 65, 85),
            ForeColor = Color.White,
            FlatStyle = WinForms.FlatStyle.Flat,
        };
        close.Click += (_, _) => Close();
        Controls.Add(close);
    }
}

/// <summary>Shared field-grid rendered by both the QR dialog and the bank-transfer dialog.</summary>
internal static class BankInfoTable
{
    public static void Render(WinForms.Form host, BankInfo info, ref int y)
    {
        Row(host, "Bank",      info.BankName,                                       copyable: false, ref y);
        Row(host, "Account #", info.AccountNumber,                                  copyable: true,  ref y);
        Row(host, "Name",      info.AccountName,                                    copyable: false, ref y);
        Row(host, "Amount",    $"{AmountString(info.Amount)} {info.Currency}",      copyable: true,  ref y);
        Row(host, "Reference", info.Reference,                                      copyable: true,  ref y,
            hint: "Must include this in the transfer description.");
    }

    private static void Row(WinForms.Form host, string label, string value,
                            bool copyable, ref int y, string? hint = null)
    {
        host.Controls.Add(new WinForms.Label
        {
            Text = label,
            Font = new Font("Segoe UI", 9),
            ForeColor = Color.LightGray,
            AutoSize = true,
            Location = new Point(16, y),
        });
        host.Controls.Add(new WinForms.TextBox
        {
            Text = value,
            ReadOnly = true,
            BackColor = Color.FromArgb(30, 41, 59),
            ForeColor = Color.White,
            BorderStyle = WinForms.BorderStyle.FixedSingle,
            Location = new Point(108, y - 2),
            Size = new Size(248, 22),
            Font = new Font("Cascadia Code", 9),
        });
        if (copyable)
        {
            var btn = new WinForms.Button
            {
                Text = "Copy",
                Location = new Point(364, y - 3),
                Size = new Size(70, 24),
                BackColor = Color.FromArgb(51, 65, 85),
                ForeColor = Color.White,
                FlatStyle = WinForms.FlatStyle.Flat,
                Font = new Font("Segoe UI", 8),
            };
            btn.Click += (_, _) => WinForms.Clipboard.SetText(value);
            host.Controls.Add(btn);
        }
        y += 28;
        if (!string.IsNullOrEmpty(hint))
        {
            host.Controls.Add(new WinForms.Label
            {
                Text = hint,
                Font = new Font("Segoe UI", 8, FontStyle.Italic),
                ForeColor = Color.LightGray,
                AutoSize = true,
                Location = new Point(108, y),
            });
            y += 18;
        }
    }

    private static string AmountString(double v) =>
        v == Math.Floor(v) ? ((long)v).ToString() : v.ToString("F2");
}
