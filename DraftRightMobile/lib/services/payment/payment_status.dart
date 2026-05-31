/// Lifecycle of a single payment, mirrored from the backend
/// `PaymentStatus` enum in
/// `backend/src/payment/entities/payment.entity.ts`.
///
/// `/payment/status/:ref` also returns the synthetic `not_found` when
/// the reference code doesn't match any row — modelled here as a
/// distinct value so the UI doesn't have to inspect a separate flag.
enum PaymentStatus {
  pending,
  completed,
  failed,
  expired,
  refunded,
  notFound,
  unknown;

  static PaymentStatus fromWire(String value) {
    switch (value) {
      case 'pending':   return PaymentStatus.pending;
      case 'completed': return PaymentStatus.completed;
      case 'failed':    return PaymentStatus.failed;
      case 'expired':   return PaymentStatus.expired;
      case 'refunded':  return PaymentStatus.refunded;
      case 'not_found': return PaymentStatus.notFound;
      default:          return PaymentStatus.unknown;
    }
  }

  /// True once the payment has reached a final state (success or
  /// failure) and shouldn't be polled further.
  bool get isTerminal {
    switch (this) {
      case PaymentStatus.pending:
      case PaymentStatus.notFound:
      case PaymentStatus.unknown:
        return false;
      case PaymentStatus.completed:
      case PaymentStatus.failed:
      case PaymentStatus.expired:
      case PaymentStatus.refunded:
        return true;
    }
  }

  bool get isSuccess => this == PaymentStatus.completed;
}

/// One snapshot returned by `/payment/status/:ref`.  Used by the
/// foreground poller so UI doesn't see raw JSON.
class PaymentStatusUpdate {
  final String referenceCode;
  final PaymentStatus status;
  final num? amount;
  final String? currency;
  final String? planName;
  final DateTime? completedAt;
  final DateTime? expiresAt;

  const PaymentStatusUpdate({
    required this.referenceCode,
    required this.status,
    this.amount,
    this.currency,
    this.planName,
    this.completedAt,
    this.expiresAt,
  });

  factory PaymentStatusUpdate.fromJson(Map<String, dynamic> j) {
    return PaymentStatusUpdate(
      referenceCode: (j['reference_code'] ?? '').toString(),
      status: PaymentStatus.fromWire((j['status'] ?? '').toString()),
      amount: j['amount'] as num?,
      currency: j['currency']?.toString(),
      planName: j['plan_name']?.toString(),
      completedAt: _parseDate(j['completed_at']),
      expiresAt: _parseDate(j['expires_at']),
    );
  }

  static DateTime? _parseDate(Object? value) {
    if (value is String && value.isNotEmpty) {
      return DateTime.tryParse(value);
    }
    return null;
  }
}
