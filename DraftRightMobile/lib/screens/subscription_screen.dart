import 'package:flutter/material.dart';
import 'package:provider/provider.dart';
import 'package:draftright_mobile/services/auth_service.dart';
import 'package:draftright_mobile/services/backend_client.dart';
import 'package:draftright_mobile/services/payment_service.dart';
import 'package:draftright_mobile/services/settings_service.dart';

class SubscriptionScreen extends StatefulWidget {
  const SubscriptionScreen({super.key});

  @override
  State<SubscriptionScreen> createState() => _SubscriptionScreenState();
}

class _SubscriptionScreenState extends State<SubscriptionScreen>
    with WidgetsBindingObserver {
  SubscriptionInfo? _info;
  bool _isLoading = true;
  String? _error;
  // True while the upgrade URL is being fetched + the browser is
  // opening. Disables the Upgrade button so double-taps can't fire
  // two checkout sessions.
  bool _upgrading = false;

  @override
  void initState() {
    super.initState();
    // Refresh subscription on app resume — covers the
    // external-browser-checkout return path:
    //   1. User taps "Upgrade" → browser opens Lemon Squeezy hosted
    //      checkout.
    //   2. User pays; LS fires the webhook to our backend; backend
    //      activates the subscription.
    //   3. User comes back to the app (manually for now; deep-link
    //      Universal Link / App Link is a follow-up).
    //   4. AppLifecycleState.resumed fires → we re-fetch /subscription.
    //
    // If the webhook hasn't landed yet (lag of a second or two), the
    // user can still pull-to-refresh; the AppBar refresh button also
    // calls _load.
    WidgetsBinding.instance.addObserver(this);
    _load();
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
      final auth = context.read<AuthService>();
      final settings = context.read<SettingsService>();
      final client = BackendClient(
        auth: auth,
        getBaseUrl: () => settings.backendUrl,
      );
      final info = await client.getSubscription();
      setState(() {
        _info = info;
        _isLoading = false;
      });
    } catch (e) {
      setState(() {
        _error = e.toString().replaceFirst('Exception: ', '');
        _isLoading = false;
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
          // "Upgrade" CTA. Wording is deliberately "Continue to website"
          // rather than "Buy" / "Subscribe" — Apple's App Store
          // reviewers nitpick the verb on link-out buttons even with
          // the External Link Account entitlement.
          FilledButton.icon(
            onPressed: _upgrading ? null : _onUpgradeTap,
            icon: _upgrading
                ? const SizedBox(
                    width: 16, height: 16,
                    child: CircularProgressIndicator(
                      strokeWidth: 2,
                      valueColor: AlwaysStoppedAnimation<Color>(Colors.white),
                    ),
                  )
                : const Icon(Icons.open_in_new),
            label: Text(_upgrading ? 'Opening checkout…' : 'Continue to website to upgrade'),
            style: FilledButton.styleFrom(
              minimumSize: const Size.fromHeight(48),
            ),
          ),
          const SizedBox(height: 12),
          Text(
            'Opens a secure checkout on draftright.info. '
            'Apple Pay / Google Pay / card all accepted. '
            'You will return here after paying.',
            style: TextStyle(color: Colors.grey.shade600, fontSize: 13),
          ),
        ],
      ],
    );
  }

  Future<void> _onUpgradeTap() async {
    if (_upgrading) return;
    setState(() => _upgrading = true);
    try {
      final auth = context.read<AuthService>();
      final settings = context.read<SettingsService>();
      final backend = BackendClient(
        auth: auth,
        getBaseUrl: () => settings.backendUrl,
      );
      final payment = PaymentService(backend);
      final launched = await payment.launchUpgrade();
      if (!mounted) return;
      if (!launched) {
        ScaffoldMessenger.of(context).showSnackBar(
          const SnackBar(
            content: Text(
              'Could not open checkout. Visit draftright.info in your browser.',
            ),
          ),
        );
      }
    } catch (e) {
      if (!mounted) return;
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(content: Text(e.toString().replaceFirst('Exception: ', ''))),
      );
    } finally {
      if (mounted) setState(() => _upgrading = false);
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
