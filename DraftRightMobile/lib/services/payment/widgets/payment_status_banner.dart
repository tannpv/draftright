import 'dart:async';
import 'package:flutter/material.dart';
import 'package:draftright_mobile/services/payment/payment_status.dart';

/// Compact live-status banner shown inside QR / bank-transfer sheets.
///
/// Subscribes to a [Stream<PaymentStatusUpdate>] (the
/// `PaymentService.watchPayment` poller) and renders one of three
/// visual states:
///
///   - **pending** — spinner + "Waiting for payment…"
///   - **success** — green check + "Payment confirmed!", then auto-
///     dismisses the enclosing route via `Navigator.maybePop` after
///     [autoPopDelay].
///   - **failure** — red icon + reason string ("Payment failed" /
///     "Took too long" / "Refunded"), stays visible until the user
///     closes the sheet manually.
///
/// A null [stream] makes the banner a no-op so the sheets render the
/// same way in tests / when the watcher is unavailable.
class PaymentStatusBanner extends StatefulWidget {
  final Stream<PaymentStatusUpdate>? stream;
  final Duration autoPopDelay;

  const PaymentStatusBanner({
    super.key,
    required this.stream,
    this.autoPopDelay = const Duration(seconds: 2),
  });

  @override
  State<PaymentStatusBanner> createState() => _PaymentStatusBannerState();
}

class _PaymentStatusBannerState extends State<PaymentStatusBanner> {
  StreamSubscription<PaymentStatusUpdate>? _sub;
  PaymentStatusUpdate? _latest;
  Timer? _autoPopTimer;

  @override
  void initState() {
    super.initState();
    final stream = widget.stream;
    if (stream != null) {
      _sub = stream.listen(_onUpdate);
    }
  }

  @override
  void dispose() {
    _sub?.cancel();
    _autoPopTimer?.cancel();
    super.dispose();
  }

  void _onUpdate(PaymentStatusUpdate u) {
    if (!mounted) return;
    setState(() => _latest = u);
    if (u.status.isSuccess) {
      // Schedule the auto-close — the user sees the green tick for
      // [autoPopDelay] before the sheet disappears, so they get
      // unambiguous confirmation before context switches back to
      // the Subscription screen.
      _autoPopTimer?.cancel();
      _autoPopTimer = Timer(widget.autoPopDelay, () {
        if (mounted) Navigator.of(context).maybePop();
      });
    }
  }

  @override
  Widget build(BuildContext context) {
    if (widget.stream == null) return const SizedBox.shrink();
    final u = _latest;
    if (u == null) {
      return _Banner(
        color: Colors.blue.shade50,
        textColor: Colors.blue.shade900,
        icon: _spinner(),
        text: 'Waiting for payment…',
      );
    }
    switch (u.status) {
      case PaymentStatus.pending:
      case PaymentStatus.notFound:
      case PaymentStatus.unknown:
        return _Banner(
          color: Colors.blue.shade50,
          textColor: Colors.blue.shade900,
          icon: _spinner(),
          text: 'Waiting for payment…',
        );
      case PaymentStatus.completed:
        return _Banner(
          color: Colors.green.shade50,
          textColor: Colors.green.shade900,
          icon: Icon(Icons.check_circle, color: Colors.green.shade700, size: 22),
          text: 'Payment confirmed!',
        );
      case PaymentStatus.failed:
        return _Banner(
          color: Colors.red.shade50,
          textColor: Colors.red.shade900,
          icon: Icon(Icons.error_outline, color: Colors.red.shade700, size: 22),
          text: 'Payment failed. Please try again.',
        );
      case PaymentStatus.expired:
        return _Banner(
          color: Colors.orange.shade50,
          textColor: Colors.orange.shade900,
          icon: Icon(Icons.hourglass_disabled, color: Colors.orange.shade700, size: 22),
          text: 'Took too long to confirm. If you already paid, '
                'check Subscription in a minute.',
        );
      case PaymentStatus.refunded:
        return _Banner(
          color: Colors.grey.shade100,
          textColor: Colors.grey.shade900,
          icon: Icon(Icons.undo, color: Colors.grey.shade700, size: 22),
          text: 'Refunded.',
        );
    }
  }

  Widget _spinner() => const SizedBox(
        width: 18,
        height: 18,
        child: CircularProgressIndicator(strokeWidth: 2),
      );
}

class _Banner extends StatelessWidget {
  final Color color;
  final Color textColor;
  final Widget icon;
  final String text;

  const _Banner({
    required this.color,
    required this.textColor,
    required this.icon,
    required this.text,
  });

  @override
  Widget build(BuildContext context) {
    return Container(
      width: double.infinity,
      padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 10),
      decoration: BoxDecoration(
        color: color,
        borderRadius: BorderRadius.circular(8),
      ),
      child: Row(
        children: [
          icon,
          const SizedBox(width: 10),
          Expanded(
            child: Text(text,
                style: TextStyle(color: textColor, fontWeight: FontWeight.w600, fontSize: 13)),
          ),
        ],
      ),
    );
  }
}
