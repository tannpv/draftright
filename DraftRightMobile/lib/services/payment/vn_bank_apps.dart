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

/// Context passed when launching.  Launchers with a known per-bank
/// deep-link format use this to prefill the transfer screen
/// (account / amount / memo).  Launchers without one fall back to
/// opening the app's home screen.
class BankAppLaunchContext {
  /// Transfer amount (integer string for VND).
  final String? amount;
  /// NAPAS bank BIN of the RECEIVING bank (e.g. '970422' = MB).
  /// Different from the launcher's own `code`; the user picks which
  /// app to open, but the money still flows to a single receiving
  /// account.
  final String? receiverBankBin;
  /// Receiving account number.
  final String? receiverAccount;
  /// Receiving account name (display only — banks ignore this on
  /// transfer screens).
  final String? receiverName;
  /// Transfer memo / reference code.  Critical: SePay webhook
  /// matches on this to auto-confirm the payment.
  final String? memo;

  const BankAppLaunchContext({
    this.amount,
    this.receiverBankBin,
    this.receiverAccount,
    this.receiverName,
    this.memo,
  });
}

/// Builder that produces a bank-specific deep-link URL from a
/// [BankAppLaunchContext].  Returns null when the context lacks
/// fields the bank's URL format requires (caller falls back to
/// home-screen launch).
typedef BankDeepLinkBuilder = String? Function(BankAppLaunchContext ctx);

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
  /// banks where ACTION_MAIN isn't enough.  Empty = no secondary
  /// attempt.  Schemes change/break silently — keep them as
  /// fallback only.
  final String urlScheme;

  /// Optional builder that produces a per-bank deep-link URL from
  /// the supplied [BankAppLaunchContext].  When set + the resulting
  /// URL launches successfully, the bank app opens **directly on
  /// the transfer screen** with account/amount/memo prefilled —
  /// the Zalo-style hand-off.  When null OR the URL can't launch,
  /// the launcher falls through to the home-screen-open path.
  final BankDeepLinkBuilder? deepLinkBuilder;

  const AndroidPackageBankAppLauncher({
    required this.code,
    required this.displayName,
    required this.androidPackage,
    this.urlScheme = '',
    this.deepLinkBuilder,
  });

  @override
  Future<BankAppLaunchOutcome> launch({BankAppLaunchContext? context}) async {
    // 1. Try the per-bank deep link.  CRITICAL: scope the intent to
    //    the bank's androidPackage so Android only fires the URL at
    //    that one app — never at the default browser.  Without the
    //    `package:` constraint, an `https://` deep link Android
    //    can't match against any intent-filter happily falls through
    //    to Chrome → user sees "This site can't be reached".
    if (deepLinkBuilder != null && context != null) {
      final url = deepLinkBuilder!(context);
      if (url != null && url.isNotEmpty) {
        try {
          await AndroidIntent(
            action: 'android.intent.action.VIEW',
            data: url,
            package: androidPackage,
            flags: <int>[Flag.FLAG_ACTIVITY_NEW_TASK],
          ).launch();
          return BankAppLaunchOutcome.appOpened;
        } catch (e) {
          if (kDebugMode) {
            debugPrint('AndroidPackageLauncher($code) deepLink miss: $e');
          }
          // fall through to home-screen launch
        }
      }
    }
    // 2. Open the app's home screen via Android Intent.  User
    //    navigates to QR scanner / transfer screen manually.
    try {
      await AndroidIntent(
        action: 'android.intent.action.MAIN',
        category: 'android.intent.category.LAUNCHER',
        package: androidPackage,
        flags: <int>[Flag.FLAG_ACTIVITY_NEW_TASK],
      ).launch();
      return BankAppLaunchOutcome.appOpened;
    } catch (e) {
      if (kDebugMode) debugPrint('AndroidPackageLauncher($code) intent miss: $e');
    }
    // 3. Tertiary: bank-specific URL scheme (no `package:` scope
    //    because some banks register a scheme without exposing a
    //    public ACTION_MAIN activity).
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
    // 4. Nothing launched — typed outcome so the caller can prompt.
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
          deepLinkBuilder: _vietqrUniversalLink,
        ),
        AndroidPackageBankAppLauncher(
          code: 'ACB',
          displayName: 'ACB ONE',
          androidPackage: 'mobile.acb.com.vn',
          deepLinkBuilder: _vietqrUniversalLink,
        ),
        AndroidPackageBankAppLauncher(
          code: 'VCB',
          displayName: 'Vietcombank',
          androidPackage: 'com.VCB',
          urlScheme: 'vcbdigibank://',
          deepLinkBuilder: _vietqrUniversalLink,
        ),
        AndroidPackageBankAppLauncher(
          code: 'AB',
          displayName: 'ABBank',
          androidPackage: 'vn.com.abbank.mobilebanking',
          urlScheme: 'abbankezpay://',
          deepLinkBuilder: _vietqrUniversalLink,
        ),
        AndroidPackageBankAppLauncher(
          code: 'TPB',
          displayName: 'TPBank',
          androidPackage: 'com.tpb.mb.gprsandroid',
          urlScheme: 'tpb://',
          deepLinkBuilder: _vietqrUniversalLink,
        ),
        AndroidPackageBankAppLauncher(
          code: 'TCB',
          displayName: 'Techcombank',
          androidPackage: 'vn.com.techcombank.bb.app',
          urlScheme: 'techcombank://',
          deepLinkBuilder: _vietqrUniversalLink,
        ),
        AndroidPackageBankAppLauncher(
          code: 'VTB',
          displayName: 'VietinBank',
          androidPackage: 'com.vietinbank.ipay',
          urlScheme: 'vietinbank://',
          deepLinkBuilder: _vietqrUniversalLink,
        ),
      ]);

  /// VietQR-hosted universal link.  When a VN bank app has
  /// registered an Android intent-filter for the `vietqr.io`
  /// domain (most major ones do as of 2026) tapping this URL opens
  /// the bank app directly on the transfer screen with
  /// account+amount+memo prefilled — same UX as Zalo.  Banks that
  /// haven't registered the filter fall back to the home-screen
  /// launch path.
  ///
  /// Format used by vietqr.io intent:
  ///   https://qr.vietqr.io/transfer?bank=<BIN>&account=<acct>&amount=<n>&memo=<m>
  static String? _vietqrUniversalLink(BankAppLaunchContext ctx) {
    final bin = ctx.receiverBankBin;
    final acct = ctx.receiverAccount;
    if (bin == null || bin.isEmpty || acct == null || acct.isEmpty) {
      return null;
    }
    final params = <String, String>{
      'bank': bin,
      'account': acct,
      if (ctx.amount != null && ctx.amount!.isNotEmpty) 'amount': ctx.amount!,
      if (ctx.memo != null && ctx.memo!.isNotEmpty) 'memo': ctx.memo!,
    };
    return Uri.https('qr.vietqr.io', '/transfer', params).toString();
  }

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
