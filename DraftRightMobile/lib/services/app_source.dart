import 'dart:io';

/// Wire values for the `source` field sent to /bug-reports and /feedback,
/// identifying which client submitted. Single source of truth (#205 #5) — the
/// bug-report and feedback services previously copy-pasted _detectSource().
const String appSourceIOS = 'ios-app';
const String appSourceAndroid = 'android-app';

/// The client-source string for the current platform. Non-mobile hosts (test
/// runner, desktop) fall back to [appSourceAndroid] — a value the backend
/// accepts — rather than throwing.
String detectAppSource() {
  try {
    if (Platform.isIOS) return appSourceIOS;
    if (Platform.isAndroid) return appSourceAndroid;
  } catch (_) {/* non-mobile path */}
  return appSourceAndroid;
}
