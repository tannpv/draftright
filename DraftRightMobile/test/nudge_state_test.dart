import 'package:flutter_test/flutter_test.dart';
import 'package:draftright_mobile/models/nudge_state.dart';

void main() {
  test('parses free_counter nudge', () {
    final n = NudgeState.fromJson({
      'tier': 'free',
      'usage_today': 7,
      'daily_limit': 10,
      'expires_at': null,
      'banner': 'free_counter',
    });
    expect(n.banner, NudgeBanner.freeCounter);
    expect(n.usageToday, 7);
    expect(n.dailyLimit, 10);
  });

  test('unknown banner falls back to none (forward-compatible)', () {
    final n = NudgeState.fromJson({'banner': 'something_new'});
    expect(n.banner, NudgeBanner.none);
  });
}
