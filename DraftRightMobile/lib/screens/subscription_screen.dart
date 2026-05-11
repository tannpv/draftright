import 'package:flutter/material.dart';
import 'package:provider/provider.dart';
import 'package:draftright_mobile/services/auth_service.dart';
import 'package:draftright_mobile/services/backend_client.dart';
import 'package:draftright_mobile/services/settings_service.dart';

class SubscriptionScreen extends StatefulWidget {
  const SubscriptionScreen({super.key});

  @override
  State<SubscriptionScreen> createState() => _SubscriptionScreenState();
}

class _SubscriptionScreenState extends State<SubscriptionScreen> {
  SubscriptionInfo? _info;
  bool _isLoading = true;
  String? _error;

  @override
  void initState() {
    super.initState();
    _load();
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
          FilledButton.icon(
            onPressed: () {
              // Future: link to app store upgrade
              ScaffoldMessenger.of(context).showSnackBar(
                const SnackBar(content: Text('Upgrade coming soon!')),
              );
            },
            icon: const Icon(Icons.upgrade),
            label: const Text('Upgrade to Pro'),
          ),
        ],
      ],
    );
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
