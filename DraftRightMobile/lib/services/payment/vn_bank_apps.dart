import 'package:flutter/foundation.dart';
import 'package:flutter/services.dart';
import 'package:url_launcher/url_launcher.dart';

/// Result of attempting to open a banking app.  UI surfaces these so
/// the user knows what happened (app launched, app missing →
/// Play Store opened, total failure).
enum BankAppLaunchOutcome {
  appOpened,
  fallbackOpened, // Play Store / web fallback opened
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

/// Concrete launcher that tries `<scheme>://` first, falls back to
/// Play Store at the package name.  Covers ~all VN banks today; if
/// a bank ships a stable Universal Link or NAPAS QR intent later,
/// drop in a different launcher implementation (no UI change).
class UrlSchemeBankAppLauncher implements BankAppLauncher {
  @override
  final String code;
  @override
  final String displayName;

  /// e.g. `mbbank://`, `acbmobile://`.  Banks don't publish official
  /// specs; values here are the most widely reported ones in VN
  /// developer community.  When a scheme stops working swap it here
  /// without touching call sites.
  final String urlScheme;

  /// Play Store package id, used for the fallback when the scheme
  /// resolves to no installed app.
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
      if (ok) return BankAppLaunchOutcome.appOpened;
    } on PlatformException catch (_) {
      // Some Android versions throw instead of returning false when
      // no activity handles the scheme — fall through to Play Store.
    } catch (e) {
      if (kDebugMode) debugPrint('UrlSchemeBankAppLauncher($code) error: $e');
    }
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
        UrlSchemeBankAppLauncher(
          code: 'MB',
          displayName: 'MB Bank',
          urlScheme: 'mbbank://',
          androidPackage: 'com.mbmobile',
        ),
        UrlSchemeBankAppLauncher(
          code: 'ACB',
          displayName: 'ACB ONE',
          urlScheme: 'acbmobile://',
          androidPackage: 'mobile.acb.com.vn',
        ),
        UrlSchemeBankAppLauncher(
          code: 'VCB',
          displayName: 'Vietcombank',
          urlScheme: 'vcbdigibank://',
          androidPackage: 'com.VCB',
        ),
        UrlSchemeBankAppLauncher(
          code: 'AB',
          displayName: 'ABBank',
          urlScheme: 'abbankezpay://',
          androidPackage: 'vn.com.abbank.mobilebanking',
        ),
        UrlSchemeBankAppLauncher(
          code: 'TPB',
          displayName: 'TPBank',
          urlScheme: 'tpb://',
          androidPackage: 'com.tpb.mb.gprsandroid',
        ),
        UrlSchemeBankAppLauncher(
          code: 'TCB',
          displayName: 'Techcombank',
          urlScheme: 'techcombank://',
          androidPackage: 'vn.com.techcombank.bb.app',
        ),
        UrlSchemeBankAppLauncher(
          code: 'VTB',
          displayName: 'VietinBank',
          urlScheme: 'vietinbank://',
          androidPackage: 'com.vietinbank.ipay',
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
