import 'package:android_intent_plus/android_intent.dart';
import 'package:android_intent_plus/flag.dart';
import 'package:flutter/foundation.dart';
import 'package:flutter/services.dart';
import 'package:url_launcher/url_launcher.dart';

/// Result of attempting to open a banking app.  UI surfaces these so
/// the user knows what happened.  Distinct outcomes keep error
/// surfaces specific — "not installed" gets a different snackbar
/// from "couldn't launch even though installed".
enum BankAppLaunchOutcome {
  appOpened,
  appNotInstalled,  // package check returned false; nothing launched
  fallbackOpened,   // user opted into Play Store install flow
  failed,
}

/// Context passed when launching — future launchers may prefill the
/// transfer details (amount, account, memo) when the target app
/// supports a structured deep link.  Today's URL-scheme launchers
/// ignore everything and just open the app; declared here so the
/// contract is ready for richer launchers without breaking callers.
class BankAppLaunchContext {
  final String? amount;
  final String? receiverBankCode;
  final String? receiverAccount;
  final String? memo;
  const BankAppLaunchContext({
    this.amount,
    this.receiverBankCode,
    this.receiverAccount,
    this.memo,
  });
}

/// One bank's "open app" capability.
///
/// **Strategy pattern**: each bank app can choose how to be opened
/// — today it's always a URL scheme + Play Store fallback, but
/// tomorrow some banks (those with Universal Links / NAPAS QR
/// intents) plug in different launchers.  UI never branches on
/// bank; it iterates and calls `.launch()`.
abstract class BankAppLauncher {
  /// Stable identifier (NAPAS bank code: 'MB', 'ACB', 'VCB', …).
  String get code;
  String get displayName;

  /// Try to open the app.  Implementations are responsible for the
  /// Play Store fallback when the app isn't installed; they SHOULD
  /// NOT throw — instead return [BankAppLaunchOutcome.failed] so the
  /// UI can render a consistent error path.
  Future<BankAppLaunchOutcome> launch({BankAppLaunchContext? context});
}

/// Launches a bank app by **Android package name** using
/// `Intent.ACTION_MAIN` + `category.LAUNCHER`.  Way more reliable
/// than custom URL schemes (banks change/break those silently and
/// `canLaunchUrl` returns false even when the app is installed,
/// firing the Play Store fallback unnecessarily — observed for
/// ACB ONE on 2026-06-01).
///
/// Falls back to a URL scheme attempt for older / un-mapped banks,
/// then to a Play Store URL **only if explicitly requested** —
/// returns `appNotInstalled` instead of redirecting blindly so the
/// UI can show a "Install <bank>?" prompt rather than yanking the
/// user out of context.
class AndroidPackageBankAppLauncher implements BankAppLauncher {
  @override
  final String code;
  @override
  final String displayName;

  /// Stable Play Store package id.
  final String androidPackage;

  /// Best-effort URL scheme used as a secondary launch attempt for
  /// banks where ACTION_MAIN isn't enough (e.g. they require a
  /// scheme-specific deep link to reach an internal screen).  Empty
  /// = no secondary attempt.
  final String urlScheme;

  const AndroidPackageBankAppLauncher({
    required this.code,
    required this.displayName,
    required this.androidPackage,
    this.urlScheme = '',
  });

  @override
  Future<BankAppLaunchOutcome> launch({BankAppLaunchContext? context}) async {
    // 1. Try launching the package directly via Android Intent.
    //    Works iff the package is installed; doesn't depend on the
    //    bank declaring a URL scheme.
    try {
      final intent = AndroidIntent(
        action: 'android.intent.action.MAIN',
        category: 'android.intent.category.LAUNCHER',
        package: androidPackage,
        flags: <int>[Flag.FLAG_ACTIVITY_NEW_TASK],
      );
      await intent.launch();
      return BankAppLaunchOutcome.appOpened;
    } catch (e) {
      if (kDebugMode) debugPrint('AndroidPackageLauncher($code) intent miss: $e');
    }
    // 2. Try the URL scheme as a secondary best-effort.  Some banks
    //    register a scheme but not a public launcher activity for
    //    ACTION_MAIN.
    if (urlScheme.isNotEmpty) {
      try {
        final uri = Uri.parse(urlScheme);
        if (await canLaunchUrl(uri)) {
          final ok = await launchUrl(uri, mode: LaunchMode.externalApplication);
          if (ok) return BankAppLaunchOutcome.appOpened;
        }
      } on PlatformException catch (_) {
        // ignore — fall through
      } catch (e) {
        if (kDebugMode) debugPrint('AndroidPackageLauncher($code) scheme miss: $e');
      }
    }
    // 3. Nothing launched.  Don't redirect to Play Store
    //    automatically — return the typed outcome so the caller
    //    can prompt the user instead.
    return BankAppLaunchOutcome.appNotInstalled;
  }

  /// Open the Play Store page for this bank.  Called explicitly by
  /// the UI when the user confirms they want to install — not in
  /// the default `launch()` path.
  Future<BankAppLaunchOutcome> openPlayStore() async {
    try {
      final ok = await launchUrl(
        Uri.parse('https://play.google.com/store/apps/details?id=$androidPackage'),
        mode: LaunchMode.externalApplication,
      );
      return ok ? BankAppLaunchOutcome.fallbackOpened : BankAppLaunchOutcome.failed;
    } catch (e) {
      if (kDebugMode) debugPrint('Play Store fallback($code) error: $e');
      return BankAppLaunchOutcome.failed;
    }
  }
}

/// Legacy URL-scheme-only launcher.  Kept for tests + non-Android
/// platforms; production code now uses [AndroidPackageBankAppLauncher].
class UrlSchemeBankAppLauncher implements BankAppLauncher {
  @override
  final String code;
  @override
  final String displayName;
  final String urlScheme;
  final String androidPackage;

  const UrlSchemeBankAppLauncher({
    required this.code,
    required this.displayName,
    required this.urlScheme,
    required this.androidPackage,
  });

  @override
  Future<BankAppLaunchOutcome> launch({BankAppLaunchContext? context}) async {
    try {
      final ok = await launchUrl(
        Uri.parse(urlScheme),
        mode: LaunchMode.externalApplication,
      );
      return ok ? BankAppLaunchOutcome.appOpened : BankAppLaunchOutcome.appNotInstalled;
    } on PlatformException catch (_) {
      return BankAppLaunchOutcome.appNotInstalled;
    } catch (e) {
      if (kDebugMode) debugPrint('UrlSchemeBankAppLauncher($code) error: $e');
      return BankAppLaunchOutcome.failed;
    }
  }
}

/// Iterable catalog of launchers.  UI asks the registry for what to
/// show; adding a new bank = one entry in `forVietnam()` (or a custom
/// factory for other locales / test fixtures).
class BankAppRegistry {
  final List<BankAppLauncher> _entries;
  const BankAppRegistry(this._entries);

  /// Built-in catalog for the Vietnamese market.  Ordered by user
  /// popularity (informed guess; reorder freely).  Adding a bank =
  /// one row; UI iterates automatically.
  factory BankAppRegistry.forVietnam() => const BankAppRegistry([
        AndroidPackageBankAppLauncher(
          code: 'MB',
          displayName: 'MB Bank',
          androidPackage: 'com.mbmobile',
          urlScheme: 'mbbank://',
        ),
        AndroidPackageBankAppLauncher(
          code: 'ACB',
          displayName: 'ACB ONE',
          androidPackage: 'mobile.acb.com.vn',
        ),
        AndroidPackageBankAppLauncher(
          code: 'VCB',
          displayName: 'Vietcombank',
          androidPackage: 'com.VCB',
          urlScheme: 'vcbdigibank://',
        ),
        AndroidPackageBankAppLauncher(
          code: 'AB',
          displayName: 'ABBank',
          androidPackage: 'vn.com.abbank.mobilebanking',
          urlScheme: 'abbankezpay://',
        ),
        AndroidPackageBankAppLauncher(
          code: 'TPB',
          displayName: 'TPBank',
          androidPackage: 'com.tpb.mb.gprsandroid',
          urlScheme: 'tpb://',
        ),
        AndroidPackageBankAppLauncher(
          code: 'TCB',
          displayName: 'Techcombank',
          androidPackage: 'vn.com.techcombank.bb.app',
          urlScheme: 'techcombank://',
        ),
        AndroidPackageBankAppLauncher(
          code: 'VTB',
          displayName: 'VietinBank',
          androidPackage: 'com.vietinbank.ipay',
          urlScheme: 'vietinbank://',
        ),
      ]);

  /// All launchers in the catalog, in display order.
  List<BankAppLauncher> all() => List.unmodifiable(_entries);

  /// Lookup by NAPAS bank code; null if not registered.
  BankAppLauncher? findByCode(String code) {
    for (final l in _entries) {
      if (l.code == code) return l;
    }
    return null;
  }
}
