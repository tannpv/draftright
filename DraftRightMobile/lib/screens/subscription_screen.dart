import 'package:flutter/material.dart';
import 'package:provider/provider.dart';
import 'package:draftright_mobile/services/auth_service.dart';
import 'package:draftright_mobile/services/backend_client.dart';
import 'package:draftright_mobile/services/payment_service.dart';
import 'package:draftright_mobile/services/payment/billing_period.dart';
import 'package:draftright_mobile/services/payment/payment_method.dart';
import 'package:draftright_mobile/services/settings_service.dart';
import 'package:draftright_mobile/widgets/billing_period_selector.dart';

class SubscriptionScreen extends StatefulWidget {
  const SubscriptionScreen({super.key});

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

  // User-selected billing cadence for the upgrade button.  Defaults
  // to monthly (lower friction, lower commitment).  Threaded into
  // `PaymentService.resolveProPlanId` so the backend creates a
  // checkout for the matching plan id.
  BillingPeriod _billingPeriod = BillingPeriod.monthly;

  @override
  void initState() {
    super.initState();
    final auth = context.read<AuthService>();
    final settings = context.read<SettingsService>();
    _backend = BackendClient(
      auth: auth,
      getBaseUrl: () => settings.backendUrl,
    );
    _payments = PaymentService(_backend);

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
    super.dispose();
  }

  @override
  void didChangeAppLifecycleState(AppLifecycleState state) {
    if (state == AppLifecycleState.resumed) {
      _load();
    }
  }

  Future<void> _load() async {
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

    String billingLabel;
    switch (info.billingPeriod) {
      case 'none': billingLabel = 'Free'; break;
      case 'monthly': billingLabel = 'Monthly'; break;
      case 'yearly': billingLabel = 'Yearly'; break;
      default: billingLabel = info.billingPeriod;
    }

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
        ] else ...[
          // Paid plan: show the Manage button so the user can cancel,
          // change plan, or update card.  Opens the LS Customer
          // Portal in the same in-app browser used for checkout.
          const SizedBox(height: 32),
          FilledButton.tonalIcon(
            onPressed: _openingPortal ? null : _onManageTap,
            icon: _openingPortal
                ? const SizedBox(
                    width: 16, height: 16,
                    child: CircularProgressIndicator(strokeWidth: 2),
                  )
                : const Icon(Icons.settings_outlined),
            label: Text(_openingPortal ? 'Opening…' : 'Manage subscription'),
            style: FilledButton.styleFrom(minimumSize: const Size.fromHeight(48)),
          ),
          const SizedBox(height: 8),
          Text(
            'Cancel, change plan, or update your payment method.',
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
