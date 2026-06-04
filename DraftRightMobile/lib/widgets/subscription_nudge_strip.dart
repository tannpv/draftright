import 'package:flutter/material.dart';
import '../models/nudge_state.dart';

/// Thin, app-wide nudge strip driven entirely by backend [NudgeState].
/// Renders nothing for NONE. Copy is derived here from the banner enum;
/// the same model serializes to macOS/Windows/Linux later.
class SubscriptionNudgeStrip extends StatelessWidget {
  final NudgeState? nudge;
  final VoidCallback onUpgrade;
  final VoidCallback? onDismiss;

  const SubscriptionNudgeStrip({
    super.key,
    required this.nudge,
    required this.onUpgrade,
    this.onDismiss,
  });

  @override
  Widget build(BuildContext context) {
    final n = nudge;
    if (n == null || n.banner == NudgeBanner.none) {
      return const SizedBox.shrink();
    }
    final theme = Theme.of(context);
    return Material(
      color: theme.colorScheme.secondaryContainer,
      child: Padding(
        padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 6),
        child: Row(
          children: [
            Expanded(
              child: Text(
                _text(n),
                style: theme.textTheme.bodySmall,
                overflow: TextOverflow.ellipsis,
              ),
            ),
            TextButton(onPressed: onUpgrade, child: Text(_cta(n.banner))),
            if (onDismiss != null)
              IconButton(
                icon: const Icon(Icons.close, size: 16),
                onPressed: onDismiss,
                tooltip: 'Hide for today',
              ),
          ],
        ),
      ),
    );
  }

  String _text(NudgeState n) {
    switch (n.banner) {
      case NudgeBanner.proExpiring:
        return 'Pro ends soon';
      case NudgeBanner.justExpired:
        return "You're on Free now — 10/day";
      case NudgeBanner.freeCounter:
        return '${n.remaining} / ${n.dailyLimit} left today';
      case NudgeBanner.none:
        return '';
    }
  }

  String _cta(NudgeBanner b) {
    switch (b) {
      case NudgeBanner.proExpiring:
        return 'Renew';
      case NudgeBanner.justExpired:
        return 'Restore Pro';
      default:
        return 'Go Pro';
    }
  }
}
