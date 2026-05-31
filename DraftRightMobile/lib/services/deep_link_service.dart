import 'dart:async';
import 'package:app_links/app_links.dart';
import 'package:draftright_mobile/services/logger_service.dart';

/// Classified deep-link event the rest of the app can react to.
///
/// Add a new variant + bump the [DeepLinkService._classify] switch
/// when introducing a new universal-link surface (e.g. email
/// verification, password reset).  Callers handle [UnknownDeepLink]
/// silently — never crash on an unrecognised path.
sealed class DeepLinkEvent {
  final Uri uri;
  const DeepLinkEvent(this.uri);
}

/// `/payment/success?ref=PAY-XXX` (paid) or `/payment/cancel`
/// (user backed out).  Mobile uses this to bring the Subscription
/// screen to the foreground and refresh after Lemon Squeezy's
/// hosted-checkout redirect.
class PaymentReturnEvent extends DeepLinkEvent {
  /// Backend-issued reference code.  Empty when LS doesn't pass it
  /// (e.g. user cancel) — the consumer can still refresh
  /// `/subscription` to pick up the latest state.
  final String referenceCode;
  /// True when the path is `/payment/success` (paid), false on
  /// `/payment/cancel`.
  final bool success;
  const PaymentReturnEvent(super.uri, this.referenceCode, {required this.success});
}

class UnknownDeepLink extends DeepLinkEvent {
  const UnknownDeepLink(super.uri);
}

/// Single source of truth for universal-link / app-link inbound URLs.
///
/// Mirrors [ShareService]'s static-class API so wiring at the top of
/// the app feels the same: `getInitialEvent()` drains cold-start,
/// `setHandler(onLink:)` subscribes warm-state.  Cancellation of the
/// underlying stream is handled by passing `setHandler(onLink: null)`.
class DeepLinkService {
  static final AppLinks _appLinks = AppLinks();
  static StreamSubscription<Uri>? _sub;

  /// Pull the URL that launched the app (if any), classify it, and
  /// return the event.  Returns null when the app was launched
  /// without a link.  Call once on cold-start.
  static Future<DeepLinkEvent?> getInitialEvent() async {
    try {
      final uri = await _appLinks.getInitialLink();
      if (uri == null) return null;
      final event = _classify(uri);
      DRLogger.log('Cold-start deep link: $uri → ${event.runtimeType}',
          category: 'DeepLinkService');
      return event;
    } catch (e) {
      DRLogger.warn('getInitialEvent failed: $e', category: 'DeepLinkService');
      return null;
    }
  }

  /// Subscribe to subsequent deep-links while the app is alive.
  /// Replaces any previously registered handler.  Pass `onLink: null`
  /// to unsubscribe.
  static void setHandler({void Function(DeepLinkEvent)? onLink}) {
    _sub?.cancel();
    if (onLink == null) {
      _sub = null;
      return;
    }
    _sub = _appLinks.uriLinkStream.listen(
      (uri) {
        try {
          final event = _classify(uri);
          DRLogger.log('Live deep link: $uri → ${event.runtimeType}',
              category: 'DeepLinkService');
          onLink(event);
        } catch (e) {
          DRLogger.warn('Deep-link handler failed: $e',
              category: 'DeepLinkService');
        }
      },
      onError: (Object e) {
        DRLogger.warn('uriLinkStream error: $e', category: 'DeepLinkService');
      },
    );
  }

  /// Map a URI to a typed event.  Pure-logic — covered by unit tests.
  static DeepLinkEvent classify(Uri uri) => _classify(uri);

  static DeepLinkEvent _classify(Uri uri) {
    final segments = uri.pathSegments;
    if (segments.length >= 2 && segments[0] == 'payment') {
      switch (segments[1]) {
        case 'success':
          return PaymentReturnEvent(
            uri,
            uri.queryParameters['ref'] ?? '',
            success: true,
          );
        case 'cancel':
          return PaymentReturnEvent(
            uri,
            uri.queryParameters['ref'] ?? '',
            success: false,
          );
      }
    }
    return UnknownDeepLink(uri);
  }
}
