import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:draftright_mobile/models/nudge_state.dart';
import 'package:draftright_mobile/widgets/subscription_nudge_strip.dart';

NudgeState _state(NudgeBanner b, {int used = 7, int limit = 10}) => NudgeState(
      tier: b == NudgeBanner.proExpiring ? 'pro' : 'free',
      usageToday: used, dailyLimit: limit, expiresAt: null, banner: b,
    );

void main() {
  Future<void> pump(WidgetTester t, NudgeState s) => t.pumpWidget(
        MaterialApp(home: Scaffold(body: SubscriptionNudgeStrip(nudge: s, onUpgrade: () {}))),
      );

  testWidgets('NONE renders nothing', (t) async {
    await pump(t, _state(NudgeBanner.none));
    expect(find.textContaining('left today'), findsNothing);
    expect(find.byType(TextButton), findsNothing);
  });

  testWidgets('FREE_COUNTER shows remaining count', (t) async {
    await pump(t, _state(NudgeBanner.freeCounter, used: 7, limit: 10));
    expect(find.textContaining('3 / 10 left today'), findsOneWidget);
  });

  testWidgets('JUST_EXPIRED shows restore copy', (t) async {
    await pump(t, _state(NudgeBanner.justExpired));
    expect(find.textContaining("You're on Free"), findsOneWidget);
  });
}
