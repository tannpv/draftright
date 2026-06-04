/// Mirrors the backend NudgeBanner enum. Unknown values map to [none]
/// so older clients stay forward-compatible with new server states.
enum NudgeBanner { none, proExpiring, justExpired, freeCounter }

NudgeBanner _bannerFrom(String? raw) {
  switch (raw) {
    case 'pro_expiring':
      return NudgeBanner.proExpiring;
    case 'just_expired':
      return NudgeBanner.justExpired;
    case 'free_counter':
      return NudgeBanner.freeCounter;
    default:
      return NudgeBanner.none;
  }
}

class NudgeState {
  final String tier;
  final int usageToday;
  final int dailyLimit;
  final String? expiresAt;
  final NudgeBanner banner;

  const NudgeState({
    required this.tier,
    required this.usageToday,
    required this.dailyLimit,
    required this.expiresAt,
    required this.banner,
  });

  bool get isUnlimited => dailyLimit == -1;
  int get remaining => isUnlimited ? -1 : (dailyLimit - usageToday).clamp(0, dailyLimit);

  factory NudgeState.fromJson(Map<String, dynamic> json) {
    return NudgeState(
      tier: (json['tier'] ?? 'free').toString(),
      usageToday: (json['usage_today'] as num?)?.toInt() ?? 0,
      dailyLimit: (json['daily_limit'] as num?)?.toInt() ?? 10,
      expiresAt: json['expires_at']?.toString(),
      banner: _bannerFrom(json['banner']?.toString()),
    );
  }
}
