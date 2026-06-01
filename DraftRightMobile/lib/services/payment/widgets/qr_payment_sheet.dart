import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:draftright_mobile/services/payment/checkout_result.dart';
import 'package:draftright_mobile/services/payment/payment_status.dart';
import 'package:draftright_mobile/services/payment/vn_bank_apps.dart';
import 'package:draftright_mobile/services/payment/widgets/payment_status_banner.dart';

/// Bottom-sheet shown for VietQR checkout.  Renders the QR image and
/// (when the backend includes them) the manual-transfer fallback
/// fields so users on PCs / phones without a camera can still pay.
///
/// When [statusStream] is provided the sheet also shows a live status
/// banner; when the SePay webhook activates the payment the banner
/// turns green and the sheet auto-dismisses.
class QrPaymentSheet extends StatelessWidget {
  final QrCheckout checkout;
  final Stream<PaymentStatusUpdate>? statusStream;
  const QrPaymentSheet({super.key, required this.checkout, this.statusStream});

  @override
  Widget build(BuildContext context) {
    final bank = checkout.bankInfo;
    return SafeArea(
      child: Padding(
        padding: const EdgeInsets.fromLTRB(24, 16, 24, 24),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            Container(
              width: 40,
              height: 4,
              margin: const EdgeInsets.only(bottom: 16),
              decoration: BoxDecoration(
                color: Colors.grey.shade300,
                borderRadius: BorderRadius.circular(2),
              ),
            ),
            Text(
              'Scan to pay',
              style: Theme.of(context).textTheme.titleLarge?.copyWith(fontWeight: FontWeight.bold),
            ),
            const SizedBox(height: 12),
            PaymentStatusBanner(stream: statusStream),
            const SizedBox(height: 8),
            Text(
              'Open your banking app and scan this QR code. '
              'Your plan activates automatically after payment.',
              textAlign: TextAlign.center,
              style: TextStyle(color: Colors.grey.shade700, fontSize: 13),
            ),
            const SizedBox(height: 16),
            // QR image — render at a fixed square size so it's
            // scannable on small phones.  The img.vietqr.io URL is
            // already a PNG/JPG.
            ClipRRect(
              borderRadius: BorderRadius.circular(12),
              child: Image.network(
                checkout.qrImageUrl,
                width: 260,
                height: 260,
                fit: BoxFit.contain,
                errorBuilder: (_, __, ___) => const _QrErrorBox(),
                loadingBuilder: (_, child, loading) {
                  if (loading == null) return child;
                  return const SizedBox(
                    width: 260, height: 260,
                    child: Center(child: CircularProgressIndicator()),
                  );
                },
              ),
            ),
            const SizedBox(height: 16),
            // Open-bank-app row — Zalo-style.  Each tile tries the
            // bank's URL scheme; if the app isn't installed it falls
            // back to the Play Store page.  Once the bank app opens
            // the user uses its built-in QR scanner to read the
            // on-screen image (or screenshots + scans from gallery).
            const _OpenBankAppRow(),
            if (bank != null) ...[
              const SizedBox(height: 24),
              const Divider(),
              const SizedBox(height: 8),
              Text(
                'Or transfer manually',
                style: Theme.of(context).textTheme.titleMedium?.copyWith(fontWeight: FontWeight.bold),
              ),
              const SizedBox(height: 12),
              _CopyRow(label: 'Bank',     value: bank.bankName,      copyable: false),
              _CopyRow(label: 'Account',  value: bank.accountNumber, copyable: true),
              _CopyRow(label: 'Name',     value: bank.accountName,   copyable: false),
              _CopyRow(label: 'Amount',
                  value: '${bank.amount.toStringAsFixed(0)} ${bank.currency}', copyable: true),
              _CopyRow(label: 'Reference', value: bank.reference,    copyable: true,
                  hint: 'Must include this in the transfer description.'),
            ],
            const SizedBox(height: 16),
            SizedBox(
              width: double.infinity,
              child: TextButton(
                onPressed: () => Navigator.of(context).pop(),
                child: const Text('Close'),
              ),
            ),
          ],
        ),
      ),
    );
  }
}

/// Horizontal scrollable row of Vietnamese banking apps.  Iterates
/// over [BankAppRegistry.forVietnam()] — adding a bank or swapping
/// a launch strategy = one entry in the registry, zero changes
/// here.  Zalo-style handoff: tap → bank app opens → user scans the
/// on-screen QR via the bank's built-in scanner.
class _OpenBankAppRow extends StatelessWidget {
  /// Caller can inject a custom registry for tests; default = VN.
  // ignore: unused_element_parameter
  final BankAppRegistry? registry;
  const _OpenBankAppRow({this.registry});

  @override
  Widget build(BuildContext context) {
    final launchers = (registry ?? BankAppRegistry.forVietnam()).all();
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Padding(
          padding: const EdgeInsets.only(left: 4, bottom: 6),
          child: Text(
            'Open your bank app',
            style: Theme.of(context).textTheme.titleSmall?.copyWith(fontWeight: FontWeight.bold),
          ),
        ),
        SizedBox(
          height: 88,
          child: ListView.separated(
            scrollDirection: Axis.horizontal,
            padding: const EdgeInsets.symmetric(horizontal: 4),
            itemCount: launchers.length,
            separatorBuilder: (_, __) => const SizedBox(width: 8),
            itemBuilder: (_, i) => _BankAppTile(launcher: launchers[i]),
          ),
        ),
      ],
    );
  }
}

class _BankAppTile extends StatefulWidget {
  final BankAppLauncher launcher;
  const _BankAppTile({required this.launcher});

  @override
  State<_BankAppTile> createState() => _BankAppTileState();
}

class _BankAppTileState extends State<_BankAppTile> {
  bool _launching = false;

  @override
  Widget build(BuildContext context) {
    return Material(
      color: Colors.transparent,
      child: InkWell(
        borderRadius: BorderRadius.circular(10),
        onTap: _launching ? null : _onTap,
        child: Container(
          width: 88,
          padding: const EdgeInsets.symmetric(vertical: 8, horizontal: 4),
          decoration: BoxDecoration(
            borderRadius: BorderRadius.circular(10),
            border: Border.all(color: Colors.grey.shade300),
          ),
          child: Column(
            mainAxisAlignment: MainAxisAlignment.center,
            children: [
              if (_launching)
                const SizedBox(
                  width: 28, height: 28,
                  child: CircularProgressIndicator(strokeWidth: 2),
                )
              else
                Icon(Icons.account_balance, size: 28, color: Colors.blue.shade700),
              const SizedBox(height: 6),
              Text(
                widget.launcher.displayName,
                maxLines: 1,
                overflow: TextOverflow.ellipsis,
                style: const TextStyle(fontSize: 11, fontWeight: FontWeight.w600),
              ),
            ],
          ),
        ),
      ),
    );
  }

  Future<void> _onTap() async {
    setState(() => _launching = true);
    try {
      final outcome = await widget.launcher.launch();
      if (!mounted) return;
      switch (outcome) {
        case BankAppLaunchOutcome.appOpened:
          // App launched — user is in bank app now.  No UI feedback.
          break;
        case BankAppLaunchOutcome.fallbackOpened:
          ScaffoldMessenger.of(context).showSnackBar(SnackBar(
            content: Text('${widget.launcher.displayName} not installed — opening Play Store.'),
          ));
          break;
        case BankAppLaunchOutcome.failed:
          ScaffoldMessenger.of(context).showSnackBar(SnackBar(
            content: Text('Could not open ${widget.launcher.displayName}.'),
          ));
          break;
      }
    } finally {
      if (mounted) setState(() => _launching = false);
    }
  }
}

class _QrErrorBox extends StatelessWidget {
  const _QrErrorBox();
  @override
  Widget build(BuildContext context) => Container(
        width: 260, height: 260,
        color: Colors.grey.shade100,
        alignment: Alignment.center,
        child: const Padding(
          padding: EdgeInsets.all(24),
          child: Text('Could not load QR. Use manual transfer below.',
              textAlign: TextAlign.center),
        ),
      );
}

class _CopyRow extends StatelessWidget {
  final String label;
  final String value;
  final bool copyable;
  final String? hint;
  const _CopyRow({
    required this.label,
    required this.value,
    required this.copyable,
    this.hint,
  });

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.symmetric(vertical: 6),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            children: [
              SizedBox(
                width: 90,
                child: Text(label, style: const TextStyle(color: Colors.grey, fontSize: 13)),
              ),
              Expanded(
                child: SelectableText(
                  value,
                  style: const TextStyle(fontWeight: FontWeight.w600),
                ),
              ),
              if (copyable)
                IconButton(
                  icon: const Icon(Icons.copy, size: 18),
                  tooltip: 'Copy',
                  onPressed: () async {
                    await Clipboard.setData(ClipboardData(text: value));
                    if (!context.mounted) return;
                    ScaffoldMessenger.of(context).showSnackBar(
                      SnackBar(content: Text('$label copied'), duration: const Duration(seconds: 1)),
                    );
                  },
                ),
            ],
          ),
          if (hint != null)
            Padding(
              padding: const EdgeInsets.only(left: 90, top: 2),
              child: Text(hint!, style: TextStyle(color: Colors.grey.shade600, fontSize: 11)),
            ),
        ],
      ),
    );
  }
}
