import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:draftright_mobile/services/payment/checkout_result.dart';
import 'package:draftright_mobile/services/payment/payment_status.dart';
import 'package:draftright_mobile/services/payment/widgets/payment_status_banner.dart';

/// Bottom-sheet shown for `bank_transfer` checkout.  Renders the
/// account fields plus a copyable reference code.  The user transfers
/// from their banking app manually; backend confirms when the
/// statement-line webhook lands.  When [statusStream] is provided the
/// sheet shows a live status banner and auto-closes on success.
///
/// Visually distinct from [QrPaymentSheet] (no QR image) but reuses
/// the same copy-row pattern.  Keep both screens in sync if the row
/// design changes.
class BankTransferSheet extends StatelessWidget {
  final BankTransferCheckout checkout;
  final Stream<PaymentStatusUpdate>? statusStream;
  const BankTransferSheet({super.key, required this.checkout, this.statusStream});

  @override
  Widget build(BuildContext context) {
    final info = checkout.info;
    return SafeArea(
      child: Padding(
        padding: const EdgeInsets.fromLTRB(24, 16, 24, 24),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Center(
              child: Container(
                width: 40,
                height: 4,
                margin: const EdgeInsets.only(bottom: 16),
                decoration: BoxDecoration(
                  color: Colors.grey.shade300,
                  borderRadius: BorderRadius.circular(2),
                ),
              ),
            ),
            Text(
              'Bank transfer',
              style: Theme.of(context).textTheme.titleLarge?.copyWith(fontWeight: FontWeight.bold),
            ),
            const SizedBox(height: 12),
            PaymentStatusBanner(stream: statusStream),
            const SizedBox(height: 8),
            Text(
              'Transfer this exact amount from any Vietnamese bank. '
              'The reference code links the payment to your account; '
              'your plan activates automatically once received.',
              style: TextStyle(color: Colors.grey.shade700, fontSize: 13),
            ),
            const SizedBox(height: 20),
            _Row(label: 'Bank',      value: info.bankName,      copyable: false),
            _Row(label: 'Account #', value: info.accountNumber, copyable: true),
            _Row(label: 'Account',   value: info.accountName,   copyable: false),
            _Row(label: 'Amount',
                value: '${info.amount.toStringAsFixed(0)} ${info.currency}', copyable: true),
            _Row(label: 'Reference', value: info.reference, copyable: true,
                hint: 'Must include this in the transfer description.'),
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

class _Row extends StatelessWidget {
  final String label;
  final String value;
  final bool copyable;
  final String? hint;
  const _Row({
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
                width: 100,
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
              padding: const EdgeInsets.only(left: 100, top: 2),
              child: Text(hint!, style: TextStyle(color: Colors.grey.shade600, fontSize: 11)),
            ),
        ],
      ),
    );
  }
}
