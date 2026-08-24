/// Single source of truth for the string keys shared between the Flutter app
/// and its native keyboard / share extensions (SharedPreferences on Android,
/// App Group UserDefaults on iOS) and the MethodChannel that bridges them.
/// The NATIVE side (Kotlin SharedSettings, iOS SharedSettings) keeps its own
/// copies of these exact strings — they cannot share code across the language
/// boundary, so keep both sides in sync by hand; this at least gives the DART
/// side one place. (#205)
class PrefsKeys {
  PrefsKeys._();

  static const accessToken = 'draftright.accessToken';
  static const refreshToken = 'draftright.refreshToken';
  static const userEmail = 'draftright.userEmail';
  static const backendUrl = 'draftright.backendUrl';
  static const translateLanguage = 'draftright.translateLanguage';
  static const enabledTones = 'draftright.enabledTones';
  static const defaultTone = 'draftright.defaultTone';
  static const bubblePresetTone = 'draftright.bubblePresetTone';
  static const floatingBubbleEnabled = 'draftright.floatingBubbleEnabled';
  static const enabledLanguageIds = 'draftright.enabledLanguageIds';
  static const activeLanguageId = 'draftright.activeLanguageId';
  static const lastSeenVersion = 'draftright.lastSeenVersion';
  static const voicePolishEnabled = 'draftright.voicePolishEnabled';
  static const oneTapTone = 'draftright.oneTapTone';
  static const errorReporterQueue = 'draftright.error_reporter.queue';
  static const deviceId = 'draftright.deviceId';
  static const extensionToken = 'draftright.extensionToken';
  static const onboardingComplete = 'draftright.onboardingComplete';
}

/// MethodChannel name bridging the app and its native extensions.
const String appGroupChannelName = 'com.draftright.v2/app_group';

/// Default keyboard language id.
const String kDefaultLanguageId = 'en';
