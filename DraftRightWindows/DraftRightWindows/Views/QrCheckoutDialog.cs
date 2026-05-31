using System;
using System.Drawing;
using System.IO;
using System.Net.Http;
using System.Threading.Tasks;
using DraftRightWindows.Models;
using WinForms = System.Windows.Forms;

namespace DraftRightWindows.Views;

/// <summary>
/// Modal dialog shown for VietQR checkout.  Renders the QR image and
/// (when included) the manual-transfer fallback fields so users on
/// PCs without a camera can still pay.  When a status stream is
/// provided, a live banner inside the dialog auto-dismisses on
/// confirmation.
/// </summary>
public sealed class QrCheckoutDialog : WinForms.Form
{
    private readonly QrCheckout _checkout;
    private readonly IObservable<PaymentStatusUpdate>? _statusStream;

    public QrCheckoutDialog(QrCheckout checkout, IObservable<PaymentStatusUpdate>? statusStream)
    {
        _checkout = checkout;
        _statusStream = statusStream;
        InitializeUI();
    }

    private void InitializeUI()
    {
        Text = "Scan to pay";
        Width = 460;
        Height = 620;
        StartPosition = WinForms.FormStartPosition.CenterParent;
        BackColor = Color.FromArgb(15, 23, 42);
        ForeColor = Color.White;
        FormBorderStyle = WinForms.FormBorderStyle.FixedDialog;
        MaximizeBox = false;

        var banner = new PaymentStatusBanner(_statusStream, onConfirmed: Close);
        Controls.Add(banner);

        var heading = new WinForms.Label
        {
            Text = "Scan to pay",
            Font = new Font("Segoe UI", 14, FontStyle.Bold),
            ForeColor = Color.White,
            AutoSize = true,
            Location = new Point(16, 56),
        };
        var blurb = new WinForms.Label
        {
            Text = "Open your banking app and scan this QR code.\n" +
                   "Your plan activates automatically after payment.",
            ForeColor = Color.LightGray,
            AutoSize = true,
            Location = new Point(16, 94),
        };

        var qr = new WinForms.PictureBox
        {
            Location = new Point(96, 140),
            Size = new Size(260, 260),
            SizeMode = WinForms.PictureBoxSizeMode.Zoom,
            BackColor = Color.FromArgb(30, 41, 59),
        };
        _ = LoadQrAsync(qr, _checkout.ImageUrl);

        Controls.Add(heading);
        Controls.Add(blurb);
        Controls.Add(qr);

        int y = 420;
        if (_checkout.BankInfo is { } bank)
        {
            var divider = new WinForms.Label
            {
                Text = "─── Or transfer manually ───",
                Font = new Font("Segoe UI", 9, FontStyle.Bold),
                ForeColor = Color.LightGray,
                AutoSize = true,
                Location = new Point(16, y),
            };
            Controls.Add(divider);
            y += 24;
            BankInfoTable.Render(this, bank, ref y);
        }

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

    private static async Task LoadQrAsync(WinForms.PictureBox box, string url)
    {
        try
        {
            using var http = new HttpClient();
            var bytes = await http.GetByteArrayAsync(url);
            if (box.IsDisposed) return;
            using var ms = new MemoryStream(bytes);
            box.Image = Image.FromStream(ms);
        }
        catch
        {
            if (box.IsDisposed) return;
            box.BeginInvoke(new Action(() =>
            {
                var lbl = new WinForms.Label
                {
                    Text = "Could not load QR.\nUse manual transfer below.",
                    ForeColor = Color.White,
                    BackColor = Color.FromArgb(30, 41, 59),
                    TextAlign = ContentAlignment.MiddleCenter,
                    Dock = WinForms.DockStyle.Fill,
                };
                box.Controls.Add(lbl);
            }));
        }
    }
}
