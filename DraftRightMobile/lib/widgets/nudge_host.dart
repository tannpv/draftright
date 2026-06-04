import 'package:flutter/material.dart';
import 'package:shared_preferences/shared_preferences.dart';
import '../models/nudge_state.dart';
import '../services/backend_client.dart';
import 'subscription_nudge_strip.dart';

/// Fetches the nudge once and renders the strip above [child], honoring a
/// dismiss-until-next-local-day flag. Drop this into any screen body so
/// the nudge is app-wide without duplicating fetch/dismiss logic.
class NudgeHost extends StatefulWidget {
  final BackendClient backend;
  final Widget child;
  final VoidCallback onUpgrade;

  const NudgeHost({
    super.key,
    required this.backend,
    required this.child,
    required this.onUpgrade,
  });

  @override
  State<NudgeHost> createState() => _NudgeHostState();
}

class _NudgeHostState extends State<NudgeHost> {
  static const _dismissKey = 'nudge_dismissed_on';
  NudgeState? _nudge;
  bool _dismissedToday = false;

  @override
  void initState() {
    super.initState();
    _load();
  }

  String _todayKey() {
    final now = DateTime.now();
    return '${now.year}-${now.month}-${now.day}';
  }

  Future<void> _load() async {
    try {
      final info = await widget.backend.getSubscription();
      final prefs = await SharedPreferences.getInstance();
      final dismissed = prefs.getString(_dismissKey) == _todayKey();
      if (!mounted) return;
      setState(() {
        _nudge = info.nudge;
        _dismissedToday = dismissed;
      });
    } catch (_) {
      // Best-effort: never block the screen on a nudge fetch.
    }
  }

  Future<void> _dismiss() async {
    final prefs = await SharedPreferences.getInstance();
    await prefs.setString(_dismissKey, _todayKey());
    if (!mounted) return;
    setState(() => _dismissedToday = true);
  }

  @override
  Widget build(BuildContext context) {
    return Column(
      children: [
        if (!_dismissedToday)
          SubscriptionNudgeStrip(
            nudge: _nudge,
            onUpgrade: widget.onUpgrade,
            onDismiss: _dismiss,
          ),
        Expanded(child: widget.child),
      ],
    );
  }
}
