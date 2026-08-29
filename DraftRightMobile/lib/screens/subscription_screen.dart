import 'package:flutter/material.dart';
import 'package:provider/provider.dart';
import 'package:draftright_mobile/services/auth_service.dart';
import 'package:draftright_mobile/services/backend_client.dart';
import 'package:draftright_mobile/services/payment_service.dart';
import 'package:draftright_mobile/services/payment/apple_iap_service.dart';
import 'package:draftright_mobile/services/payment/apple_products.dart';
import 'package:draftright_mobile/services/payment/billing_period.dart';
import 'package:draftright_mobile/services/payment/payment_method.dart';
import 'package:draftright_mobile/services/settings_service.dart';
import 'package:draftright_mobile/widgets/billing_period_selector.dart';

class SubscriptionScreen extends StatefulWidget {
  /// [backend] / [iapService] are test-only injection points — production
  /// callers never pass them, so `initState` builds the real instances.
  const SubscriptionScreen({super.key, this.backend, this.iapService});

  final BackendClient? backend;
  final AppleIapService? iapService;

  @override
  State<SubscriptionScreen> createState() => _SubscriptionScreenState();
}

class _SubscriptionScreenState extends State<SubscriptionScreen>
    with WidgetsBindingObserver {
  // Services constructed once in initState so we don't churn HTTP
  // clients on every rebuild.  PaymentService wraps BackendClient
  // and owns the handler map.
  late final BackendClient _backend;
  late final PaymentService _payments;
  late final AppleIapService _iap;

  SubscriptionInfo? _info;
  bool _isLoading = true;
  String? _error;

  /// Methods the user can pick from. Null = not yet loaded.
  List<PaymentMethodKind>? _methods;
  Object? _methodsError;

  // True while a checkout is being created + the handler is opening
  // its UI.  Disables the buttons so double-taps don't spawn two
  // payment intents.
  bool _starting = false;
  PaymentMethodKind? _startingKind;

  // True while we're fetching the customer-portal URL and launching
  // the browser.  Prevents double-tap.
  bool _openingPortal = false;

  // True while the in-app cancel request is in flight.  Prevents
  // double-tap that would race two DELETE /payment/subscription
  // requests.
  bool _cancelling = false;

  // User-selected billing cadence for the upgrade button.  Defaults
  // to monthly (lower friction, lower commitment).  Threaded into
  // `PaymentService.resolveProPlanId` so the backend creates a
  // checkout for the matching plan id.
  BillingPeriod _billingPeriod = BillingPeriod.monthly;

  // True while a StoreKit buy/restore call is in flight. Disables the
  // buy/restore buttons so a double-tap doesn't fire two StoreKit
  // requests.
  bool _iapBusy = false;

  @override
  void initState() {
    super.initState();
    _backend = widget.backend ??
        BackendClient(
          auth: context.read<AuthService>(),
          getBaseUrl: () => context.read<SettingsService>().backendUrl,
        );
    _payments = PaymentService(_backend);
    _iap = widget.iapService ??
        AppleIapService(_backend, onEntitlementChanged: _load);

    // Refresh subscription on app resume — covers the
    // external-browser return path:
    //   1. User taps a method → handler opens browser / sheet.
    //   2. Payment completes; backend webhook activates the plan.
    //   3. User returns to the app.
    //   4. AppLifecycleState.resumed → re-fetch /subscription.
    WidgetsBinding.instance.addObserver(this);
    _load();
    _loadMethods();
  }

  @override
  void dispose() {
    WidgetsBinding.instance.removeObserver(this);
    _iap.dispose();
    super.dispose();
  }

  @override
  void didChangeAppLifecycleState(AppLifecycleState state) {
    if (state == AppLifecycleState.resumed) {
      _load();
    }
  }

  Future<void> _load() async {
    if (!mounted) return;
    setState(() {
      _isLoading = true;
      _error = null;
    });
    try {
      final info = await _backend.getSubscription();
      if (!mounted) return;
      setState(() {
        _info = info;
        _isLoading = false;
      });
    } catch (e) {
      if (!mounted) return;
      setState(() {
        _error = e.toString().replaceFirst('Exception: ', '');
        _isLoading = false;
      });
    }
  }

  Future<void> _loadMethods() async {
    try {
      final methods = await _payments.listAvailableMethods();
      if (!mounted) return;
      setState(() {
        _methods = methods;
        _methodsError = null;
      });
    } catch (e) {
      if (!mounted) return;
      setState(() {
        _methods = const [];
        _methodsError = e;
      });
    }
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(
        title: const Text('Subscription'),
        actions: [
          IconButton(onPressed: _load, icon: const Icon(Icons.refresh)),
        ],
      ),
      body: _isLoading
          ? const Center(child: CircularProgressIndicator())
          : _error != null
              ? Center(
                  child: Padding(
                    padding: const EdgeInsets.all(24),
                    child: Column(
                      mainAxisSize: MainAxisSize.min,
                      children: [
                        const Icon(Icons.error_outline, size: 48, color: Colors.red),
                        const SizedBox(height: 16),
                        Text(_error!, textAlign: TextAlign.center, style: const TextStyle(color: Colors.red)),
                        const SizedBox(height: 16),
                        FilledButton(onPressed: _load, child: const Text('Retry')),
                      ],
                    ),
                  ),
                )
              : _buildInfo(_info!),
    );
  }

  Widget _buildInfo(SubscriptionInfo info) {
    final usagePct = info.dailyLimit > 0 ? info.usageToday / info.dailyLimit : 0.0;
    final isAtLimit = info.usageToday >= info.dailyLimit;

    String statusLabel;
    switch (info.status) {
      case 'active': statusLabel = 'Active'; break;
      case 'expired': statusLabel = 'Expired'; break;
      case 'cancelled': statusLabel = 'Cancelled'; break;
      default: statusLabel = info.status;
    }

    final bp = BillingPeriod.fromWire(info.billingPeriod);
    final String billingLabel = bp?.displayName ?? (info.isFree ? 'Free' : info.billingPeriod);

    return ListView(
      padding: const EdgeInsets.all(24),
      children: [
        _InfoCard(
          icon: Icons.workspace_premium,
          iconColor: info.isFree ? Colors.grey : Colors.amber,
          title: 'Plan',
          value: info.planName,
        ),
        const SizedBox(height: 16),
        _InfoCard(
          icon: Icons.receipt_long,
          iconColor: Colors.blue,
          title: 'Billing',
          value: billingLabel,
        ),
        const SizedBox(height: 16),
        _InfoCard(
          icon: Icons.check_circle_outline,
          iconColor: info.status == 'active' ? Colors.green : Colors.orange,
          title: 'Status',
          value: statusLabel,
        ),
        if (info.expiresAt != null) ...[
          const SizedBox(height: 16),
          _InfoCard(
            icon: Icons.calendar_today,
            iconColor: Colors.blue,
            title: 'Expires At',
            value: info.expiresAt!,
          ),
        ],
        const SizedBox(height: 24),
        Text(
          'Daily Usage',
          style: Theme.of(context).textTheme.titleMedium?.copyWith(fontWeight: FontWeight.bold),
        ),
        const SizedBox(height: 12),
        LinearProgressIndicator(
          value: usagePct.clamp(0.0, 1.0),
          minHeight: 10,
          backgroundColor: Colors.grey.shade200,
          valueColor: AlwaysStoppedAnimation<Color>(isAtLimit ? Colors.red : Colors.blue),
        ),
        const SizedBox(height: 8),
        Row(
          mainAxisAlignment: MainAxisAlignment.spaceBetween,
          children: [
            Text('${info.usageToday} used today',
                style: TextStyle(color: isAtLimit ? Colors.red : null)),
            Text('Limit: ${info.dailyLimit}', style: const TextStyle(color: Colors.grey)),
          ],
        ),
        if (info.isFree) ...[
          const SizedBox(height: 32),
          if (PaymentService.inAppCheckoutAllowed) ...[
            Text(
              'Upgrade to Pro',
              style: Theme.of(context).textTheme.titleMedium?.copyWith(fontWeight: FontWeight.bold),
            ),
            const SizedBox(height: 8),
            Text(
              'Pick a billing cadence, then a payment method. Your plan activates automatically once payment completes.',
              style: TextStyle(color: Colors.grey.shade700, fontSize: 13),
            ),
            const SizedBox(height: 16),
            BillingPeriodSelector(
              value: _billingPeriod,
              onChanged: (p) => setState(() => _billingPeriod = p),
            ),
            const SizedBox(height: 16),
            ..._buildPaymentMethodTiles(),
          ] else if (PaymentService.appleIapAllowed) ...[
            // iOS: App Store Guideline 3.1.1 requires digital subscriptions
            // to be purchasable in-app via StoreKit — buy through
            // AppleIapService, not an external checkout tile.
            Text(
              'Upgrade to Pro',
              style: Theme.of(context).textTheme.titleMedium?.copyWith(fontWeight: FontWeight.bold),
            ),
            const SizedBox(height: 8),
            BillingPeriodSelector(
              value: _billingPeriod,
              onChanged: (p) => setState(() => _billingPeriod = p),
            ),
            const SizedBox(height: 16),
            FilledButton(
              onPressed: _iapBusy ? null : _onIapBuy,
              style: FilledButton.styleFrom(minimumSize: const Size.fromHeight(48)),
              child: Text(_iapBusy ? 'Processing…' : 'Upgrade to Pro (${_billingPeriod.displayName})'),
            ),
            const SizedBox(height: 8),
            TextButton(
              onPressed: _iapBusy ? null : _onIapRestore,
              child: const Text('Restore Purchases'),
            ),
          ],
        ] else ...[
          // Paid plan: two actions.  90% of users coming here just
          // want to cancel — that's an in-app POST now (no portal
          // login dance).  Card-update + plan-change are rarer
          // actions; those still go through the LS portal.
          const SizedBox(height: 32),
          FilledButton.tonalIcon(
            onPressed: _cancelling ? null : _onCancelTap,
            icon: _cancelling
                ? const SizedBox(
                    width: 16, height: 16,
                    child: CircularProgressIndicator(strokeWidth: 2),
                  )
                : const Icon(Icons.cancel_outlined),
            label: Text(_cancelling ? 'Cancelling…' : 'Cancel subscription'),
            style: FilledButton.styleFrom(minimumSize: const Size.fromHeight(48)),
          ),
          const SizedBox(height: 8),
          TextButton.icon(
            onPressed: _openingPortal ? null : _onManageTap,
            icon: _openingPortal
                ? const SizedBox(
                    width: 14, height: 14,
                    child: CircularProgressIndicator(strokeWidth: 2),
                  )
                : const Icon(Icons.open_in_new, size: 18),
            label: Text(_openingPortal ? 'Opening…' : 'Update card or change plan'),
          ),
          const SizedBox(height: 8),
          Text(
            'Cancelling stops the next renewal — you keep Pro until your current period ends.',
            style: TextStyle(color: Colors.grey.shade600, fontSize: 12),
          ),
        ],
      ],
    );
  }

  List<Widget> _buildPaymentMethodTiles() {
    if (_methods == null) {
      return [
        const Padding(
          padding: EdgeInsets.symmetric(vertical: 16),
          child: Center(child: CircularProgressIndicator()),
        ),
      ];
    }
    if (_methodsError != null && (_methods?.isEmpty ?? true)) {
      return [
        Padding(
          padding: const EdgeInsets.symmetric(vertical: 16),
          child: Column(
            children: [
              Text('Could not load payment methods.',
                  style: TextStyle(color: Colors.red.shade700)),
              const SizedBox(height: 8),
              TextButton(onPressed: _loadMethods, child: const Text('Retry')),
            ],
          ),
        ),
      ];
    }
    if (_methods!.isEmpty) {
      return [
        Padding(
          padding: const EdgeInsets.symmetric(vertical: 16),
          child: Text('No payment methods are enabled yet. Please check back later.',
              style: TextStyle(color: Colors.grey.shade700)),
        ),
      ];
    }
    return _methods!
        .map((kind) => _PaymentMethodTile(
              descriptor: PaymentMethodDescriptor.forKind(kind),
              loading: _starting && _startingKind == kind,
              disabled: _starting,
              onTap: () => _onMethodTap(kind),
            ))
        .toList();
  }

  /// Buys the product for the selected [_billingPeriod] via StoreKit.
  /// Completion + the entitlement refresh happen out-of-band through
  /// `AppleIapService`'s `onEntitlementChanged` callback (wired to
  /// `_load` in `initState`) once the purchase redeems with the
  /// backend — this only drives the StoreKit sheet.
  Future<void> _onIapBuy() async {
    final id = AppleProducts.idFor(_billingPeriod);
    if (id == null) {
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          const SnackBar(content: Text('Purchase failed. Please try again.')),
        );
      }
      return;
    }
    setState(() => _iapBusy = true);
    try {
      await _iap.buy(id);
    } catch (e) {
      if (!mounted) return;
      ScaffoldMessenger.of(context).showSnackBar(
        const SnackBar(content: Text('Purchase failed. Please try again.')),
      );
    } finally {
      if (mounted) setState(() => _iapBusy = false);
    }
  }

  /// Restores previously-purchased StoreKit transactions — required by
  /// Apple for non-consumable/subscription IAP. Same completion path
  /// as [_onIapBuy]: `onEntitlementChanged` refreshes the screen once
  /// the restored transaction redeems with the backend.
  Future<void> _onIapRestore() async {
    setState(() => _iapBusy = true);
    try {
      await _iap.restore();
    } catch (e) {
      if (!mounted) return;
      ScaffoldMessenger.of(context).showSnackBar(
        const SnackBar(content: Text('Restore failed. Please try again.')),
      );
    } finally {
      if (mounted) setState(() => _iapBusy = false);
    }
  }

  Future<void> _onManageTap() async {
    if (_openingPortal) return;
    setState(() => _openingPortal = true);
    try {
      await _payments.openCustomerPortal();
    } catch (e) {
      if (!mounted) return;
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(content: Text(e.toString().replaceFirst('Exception: ', ''))),
      );
    } finally {
      if (mounted) setState(() => _openingPortal = false);
    }
  }

  Future<void> _onCancelTap() async {
    if (_cancelling) return;
    final accessUntilRaw = _info?.expiresAt;
    final accessUntil = accessUntilRaw != null ? DateTime.tryParse(accessUntilRaw) : null;
    final confirmed = await showDialog<bool>(
      context: context,
      builder: (ctx) => AlertDialog(
        title: const Text('Cancel subscription?'),
        content: Text(
          accessUntil != null
              ? 'Pro access will continue until ${_formatDate(accessUntil)}, '
                'after which you’ll be moved to the Free plan.'
              : 'Pro access will continue until the end of your current '
                'period, after which you’ll be moved to the Free plan.',
        ),
        actions: [
          TextButton(
            onPressed: () => Navigator.of(ctx).pop(false),
            child: const Text('Keep Pro'),
          ),
          FilledButton.tonal(
            onPressed: () => Navigator.of(ctx).pop(true),
            child: const Text('Cancel subscription'),
          ),
        ],
      ),
    );
    if (confirmed != true) return;
    if (!mounted) return;

    setState(() => _cancelling = true);
    try {
      final result = await _payments.cancelSubscription();
      if (!mounted) return;
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(
          content: Text(
            result.accessUntil != null
                ? 'Cancelled. You keep Pro until ${_formatDate(result.accessUntil!)}.'
                : 'Cancelled. You keep Pro until your current period ends.',
          ),
          duration: const Duration(seconds: 5),
        ),
      );
      await _load();
    } catch (e) {
      if (!mounted) return;
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(content: Text(e.toString().replaceFirst('Exception: ', ''))),
      );
    } finally {
      if (mounted) setState(() => _cancelling = false);
    }
  }

  /// Short human date for cancel confirmations + snackbars.  Lives
  /// here rather than a date-utility because it's the only place
  /// the file formats a date today; pull out if a second caller
  /// arrives.
  String _formatDate(DateTime d) {
    const months = [
      'Jan', 'Feb', 'Mar', 'Apr', 'May', 'Jun',
      'Jul', 'Aug', 'Sep', 'Oct', 'Nov', 'Dec',
    ];
    return '${d.day} ${months[d.month - 1]} ${d.year}';
  }

  Future<void> _onMethodTap(PaymentMethodKind kind) async {
    if (_starting) return;
    setState(() {
      _starting = true;
      _startingKind = kind;
    });
    try {
      // Pass the method so the resolver picks a currency-compatible
      // plan (VND for VietQR/bank, USD for LS/Stripe/PayPal).
      // Without this, VietQR would pick the USD Pro plan and the
      // QR code would encode amount=499 đồng (~$0.02).
      //
      // Pass billingPeriod so the resolver hits the cadence the user
      // selected.  This is the third leg of the LS yearly-fix tripod:
      // mobile sends the correct plan_id → backend locks LS to a
      // single variant → webhook re-resolves on actual charged
      // variant.  See [[project_cc_payment_lemonsqueezy]].
      final planId = await _payments.resolveProPlanId(
        method: kind,
        billingPeriod: _billingPeriod,
      );
      if (!mounted) return;
      await _payments.upgradeWith(
        context: context,
        planId: planId,
        method: kind,
      );
    } catch (e) {
      if (!mounted) return;
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(content: Text(e.toString().replaceFirst('Exception: ', ''))),
      );
    } finally {
      if (mounted) {
        setState(() {
          _starting = false;
          _startingKind = null;
        });
      }
    }
  }
}

class _PaymentMethodTile extends StatelessWidget {
  final PaymentMethodDescriptor? descriptor;
  final bool loading;
  final bool disabled;
  final VoidCallback onTap;

  const _PaymentMethodTile({
    required this.descriptor,
    required this.loading,
    required this.disabled,
    required this.onTap,
  });

  @override
  Widget build(BuildContext context) {
    if (descriptor == null) return const SizedBox.shrink();
    final d = descriptor!;
    return Padding(
      padding: const EdgeInsets.only(bottom: 12),
      child: Material(
        color: Theme.of(context).colorScheme.surface,
        borderRadius: BorderRadius.circular(12),
        child: InkWell(
          borderRadius: BorderRadius.circular(12),
          onTap: disabled ? null : onTap,
          child: Container(
            padding: const EdgeInsets.all(16),
            decoration: BoxDecoration(
              borderRadius: BorderRadius.circular(12),
              border: Border.all(color: Colors.grey.shade200),
            ),
            child: Row(
              children: [
                Icon(_iconFor(d.kind), size: 28, color: Colors.blue),
                const SizedBox(width: 16),
                Expanded(
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Text(d.displayName,
                          style: const TextStyle(fontWeight: FontWeight.bold, fontSize: 15)),
                      const SizedBox(height: 2),
                      Text(d.description,
                          style: TextStyle(color: Colors.grey.shade700, fontSize: 12)),
                    ],
                  ),
                ),
                if (loading)
                  const SizedBox(
                    width: 18, height: 18,
                    child: CircularProgressIndicator(strokeWidth: 2),
                  )
                else
                  const Icon(Icons.chevron_right, color: Colors.grey),
              ],
            ),
          ),
        ),
      ),
    );
  }

  IconData _iconFor(PaymentMethodKind kind) {
    switch (kind) {
      case PaymentMethodKind.lemonsqueezy: return Icons.credit_card;
      case PaymentMethodKind.stripe:       return Icons.credit_card;
      case PaymentMethodKind.paypal:       return Icons.account_balance_wallet;
      case PaymentMethodKind.vietqr:       return Icons.qr_code_2;
      case PaymentMethodKind.bankTransfer: return Icons.account_balance;
      case PaymentMethodKind.applePay:     return Icons.apple;
      case PaymentMethodKind.appleIap:     return Icons.shopping_bag;
      case PaymentMethodKind.googlePay:    return Icons.g_mobiledata;
    }
  }
}

class _InfoCard extends StatelessWidget {
  final IconData icon;
  final Color iconColor;
  final String title;
  final String value;

  const _InfoCard({
    required this.icon,
    required this.iconColor,
    required this.title,
    required this.value,
  });

  @override
  Widget build(BuildContext context) {
    return Container(
      padding: const EdgeInsets.all(16),
      decoration: BoxDecoration(
        borderRadius: BorderRadius.circular(12),
        border: Border.all(color: Colors.grey.shade200),
        color: Theme.of(context).colorScheme.surface,
      ),
      child: Row(
        children: [
          Icon(icon, color: iconColor, size: 28),
          const SizedBox(width: 16),
          Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Text(title, style: const TextStyle(color: Colors.grey, fontSize: 12)),
              Text(value, style: const TextStyle(fontWeight: FontWeight.bold, fontSize: 16)),
            ],
          ),
        ],
      ),
    );
  }
}
