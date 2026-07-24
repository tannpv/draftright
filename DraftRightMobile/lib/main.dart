import 'dart:async';

import 'package:flutter/foundation.dart';
import 'package:flutter/material.dart';
import 'package:package_info_plus/package_info_plus.dart';
import 'package:provider/provider.dart';
import 'package:shared_preferences/shared_preferences.dart';
import 'package:draftright_mobile/services/auth_service.dart';
import 'package:draftright_mobile/services/backend_client.dart';
import 'package:draftright_mobile/services/logger_service.dart';
import 'package:draftright_mobile/services/settings_service.dart';
import 'package:draftright_mobile/screens/login_screen.dart';
import 'package:draftright_mobile/screens/onboarding_screen.dart';
import 'package:draftright_mobile/screens/settings_screen.dart';
import 'package:draftright_mobile/screens/playground_screen.dart';
import 'package:draftright_mobile/screens/subscription_screen.dart';
import 'package:draftright_mobile/services/deep_link_service.dart';
import 'package:draftright_mobile/services/share_service.dart';
import 'package:draftright_mobile/services/error_reporter.dart';
import 'package:draftright_mobile/widgets/error_notice_overlay.dart';

// Desktop imports — only compiled on desktop platforms
import 'package:draftright_mobile/desktop/desktop_app.dart'
    if (dart.library.html) 'package:draftright_mobile/desktop/desktop_app_stub.dart';

bool get isDesktop =>
    !kIsWeb &&
    (defaultTargetPlatform == TargetPlatform.windows ||
     defaultTargetPlatform == TargetPlatform.linux ||
     defaultTargetPlatform == TargetPlatform.macOS);

/// Entry point.
///
/// Renders a splash *immediately* — no `await` before `runApp` — so a slow
/// or hanging platform-channel call during startup can never leave the user
/// staring at a blank screen. (App Store Connect rejected build 31 / 2.2.2
/// under Guideline 2.1(a) for exactly that: "Upon launching the app, a blank
/// screen is displayed." The old `main()` did six sequential `await`s before
/// `runApp`.) All real init now happens in [_Bootstrap], after the first
/// frame, with each step guarded by a timeout.
void main() {
  runZonedGuarded(() {
    WidgetsFlutterBinding.ensureInitialized();
    runApp(const _BootstrapApp());
  }, (error, stack) {
    // Best-effort — ErrorReporter may not be attached yet this early.
    try {
      ErrorReporter.reportHandled(error, stack: stack, severity: 'fatal');
    } catch (_) {/* swallow — never let the reporter crash startup */}
  });
}

const Duration _bootstrapStepTimeout = Duration(seconds: 8);

class _BootstrapApp extends StatelessWidget {
  const _BootstrapApp();

  @override
  Widget build(BuildContext context) {
    return MaterialApp(
      title: 'DraftRight',
      debugShowCheckedModeBanner: false,
      theme: ThemeData(
        colorScheme: ColorScheme.fromSeed(seedColor: Colors.blue),
        useMaterial3: true,
      ),
      darkTheme: ThemeData(
        colorScheme: ColorScheme.fromSeed(
            seedColor: Colors.blue, brightness: Brightness.dark),
        useMaterial3: true,
      ),
      // Intentionally NO ErrorNoticeOverlay here. The splash phase has no
      // Scaffold (and crucially no MultiProvider — AuthService isn't bound
      // yet), so a REPORT tap from the overlay would crash with
      // ProviderNotFoundException. Bootstrap errors still auto-submit to
      // /errors via ErrorReporter; the overlay is only wired into the inner
      // MaterialApp once Provider scope is up.
      home: const _Bootstrap(),
    );
  }
}

/// Runs all startup init *after* the first frame, swapping in the real app
/// (or, in the worst case, a "couldn't start" screen) when done. Never a
/// blank screen: the splash below is the floor of what the user can see.
class _Bootstrap extends StatefulWidget {
  const _Bootstrap();

  @override
  State<_Bootstrap> createState() => _BootstrapState();
}

class _BootstrapState extends State<_Bootstrap> {
  SettingsService? _settings;
  AuthService? _auth;
  bool _failed = false;

  @override
  void initState() {
    super.initState();
    WidgetsBinding.instance.addPostFrameCallback((_) => _init());
  }

  /// Run one init step with a hard timeout. Failures are logged + reported
  /// but never abort startup — a degraded app beats a blank screen.
  Future<void> _step(String label, Future<void> Function() body) async {
    try {
      await body().timeout(_bootstrapStepTimeout);
    } catch (e, st) {
      try {
        DRLogger.warn('bootstrap step "$label" failed: $e', category: 'APP');
      } catch (_) {}
      try {
        ErrorReporter.reportHandled(e,
            stack: st,
            severity: 'warning',
            context: {'bootstrap_step': label});
      } catch (_) {}
    }
  }

  Future<void> _init() async {
    try {
      await _step('logger', () => DRLogger.init());
      try {
        DRLogger.log('App started', category: 'APP');
      } catch (_) {}

      final settings = SettingsService();
      await _step('settings', () => settings.init());

      // Apply admin-controlled log verbosity (best-effort, one /health fetch).
      await _step('loglevel',
          () => BackendClient.applyClientLogLevel(settings.backendUrl));

      final auth = AuthService();
      await _step('auth', () => auth.init(settings.backendUrl));

      String? initialToken;
      await _step('token', () async {
        initialToken = await auth.getAccessToken();
      });

      auth.addListener(() async {
        // getAccessToken() throws "Not logged in" when no token is present
        // (expected state after logout). Swallow that locally — letting the
        // throw escape the listener turned every notifyListeners() into an
        // unhandled microtask exception, which PlatformDispatcher.onError
        // funnelled back into ErrorReporter on a hot path. Catch it and just
        // null the bearer; sign-in will refill it on success.
        try {
          final token = await auth.getAccessToken();
          ErrorReporter.setBearerToken(token);
        } catch (_) {
          ErrorReporter.setBearerToken(null);
        }
      });

      // Wire crash reporting now that we know the backend URL. Synchronous
      // and non-blocking — see ErrorReporter.attach.
      try {
        ErrorReporter.attach(
          backendUrl: settings.backendUrl,
          bearerToken: initialToken,
        );
      } catch (_) {/* reporter must never block startup */}

      // Keep ErrorReporter pointed at whichever backend the user is
      // currently using.  Without this, auto-captured errors keep
      // posting to the URL captured at boot — so swapping
      // Settings → Server from prod to dev would leak dev crashes
      // into the prod /errors stream until the app restarted.
      settings.addListener(() {
        try {
          ErrorReporter.setBackendUrl(settings.backendUrl);
        } catch (_) {/* reporter must never break a settings save */}
        try {
          // AuthService caches the base URL set in init(); it has a
          // setter, but no one was calling it.  Without this, the
          // Server toggle on LoginScreen / Settings updates the
          // SharedPreferences entry but login keeps hitting the old
          // URL (default = prod) until app restart.
          auth.setBaseUrl(settings.backendUrl);
        } catch (_) {/* same isolation rule as ErrorReporter */}
      });

      if (!mounted) return;
      setState(() {
        _settings = settings;
        _auth = auth;
      });
    } catch (e, st) {
      // Should be unreachable (every step is already guarded) — but if the
      // bootstrap itself throws, show a retry screen, never a blank one.
      try {
        DRLogger.error('bootstrap failed catastrophically: $e\n$st',
            category: 'APP');
      } catch (_) {}
      if (mounted) setState(() => _failed = true);
    }
  }

  @override
  Widget build(BuildContext context) {
    final settings = _settings;
    final auth = _auth;
    if (settings != null && auth != null) {
      return isDesktop
          ? DesktopApp(settings: settings, auth: auth)
          : DraftRightApp(settings: settings, auth: auth);
    }
    if (_failed) {
      return Scaffold(
        body: Center(
          child: Padding(
            padding: const EdgeInsets.all(24),
            child: Column(
              mainAxisSize: MainAxisSize.min,
              children: [
                const Icon(Icons.error_outline, size: 56),
                const SizedBox(height: 16),
                const Text("DraftRight couldn't start",
                    style: TextStyle(fontSize: 18, fontWeight: FontWeight.w600)),
                const SizedBox(height: 8),
                const Text('Please check your connection and try again.',
                    textAlign: TextAlign.center),
                const SizedBox(height: 20),
                FilledButton(
                  onPressed: () {
                    setState(() => _failed = false);
                    WidgetsBinding.instance
                        .addPostFrameCallback((_) => _init());
                  },
                  child: const Text('Retry'),
                ),
              ],
            ),
          ),
        ),
      );
    }
    // Splash — the worst case the reviewer (or any user) can ever see.
    return const Scaffold(
      body: Center(
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            Icon(Icons.edit_note, size: 64),
            SizedBox(height: 20),
            SizedBox(
              width: 28,
              height: 28,
              child: CircularProgressIndicator(strokeWidth: 3),
            ),
          ],
        ),
      ),
    );
  }
}

// ── Mobile app ───────────────────────────────────────────────────────────────

class DraftRightApp extends StatelessWidget {
  final SettingsService settings;
  final AuthService auth;
  const DraftRightApp({super.key, required this.settings, required this.auth});

  @override
  Widget build(BuildContext context) {
    return MultiProvider(
      providers: [
        ChangeNotifierProvider.value(value: settings),
        ChangeNotifierProvider.value(value: auth),
      ],
      child: MaterialApp(
        title: 'DraftRight',
        theme: ThemeData(
          colorScheme: ColorScheme.fromSeed(seedColor: Colors.blue),
          useMaterial3: true,
        ),
        darkTheme: ThemeData(
          colorScheme: ColorScheme.fromSeed(
              seedColor: Colors.blue, brightness: Brightness.dark),
          useMaterial3: true,
        ),
        builder: (ctx, child) =>
            ErrorNoticeOverlay(child: child ?? const SizedBox()),
        home: const HomeScreen(),
      ),
    );
  }
}

class HomeScreen extends StatefulWidget {
  const HomeScreen({super.key});

  @override
  State<HomeScreen> createState() => _HomeScreenState();
}

class _HomeScreenState extends State<HomeScreen> {
  bool _onboardingComplete = false;
  int _currentIndex = 0;

  @override
  void initState() {
    super.initState();
    _checkOnboarding();
    _restoreFloatingBubble();
    _wireDeepLinks();
    // Defer until the tree (and a Navigator) is mounted.
    WidgetsBinding.instance.addPostFrameCallback((_) => _checkWhatsNew());
  }

  /// One-time post-update "What's New": if the running version changed since
  /// last launch, fetch the notes the backend advertises for it and show them
  /// once. Records the version immediately so it can't repeat; silent on fresh
  /// installs and when no matching notes exist.
  Future<void> _checkWhatsNew() async {
    if (!mounted) return;
    final settings = context.read<SettingsService>();
    String version;
    try {
      version = (await PackageInfo.fromPlatform()).version;
    } catch (_) {
      return;
    }
    final lastSeen = settings.lastSeenVersion;
    if (lastSeen == version) return;
    await settings.setLastSeenVersion(version);
    if (lastSeen.isEmpty) return; // fresh install — nothing to announce

    final platform = _platformKey();
    if (platform == null) return;
    final notes = await BackendClient.releaseNotesForVersion(
        settings.backendUrl, platform, version);
    if (notes == null || !mounted) return;

    await showDialog<void>(
      context: context,
      builder: (ctx) => AlertDialog(
        title: Text("What's new in v$version"),
        content: SingleChildScrollView(child: Text(notes)),
        actions: [
          FilledButton(
            onPressed: () => Navigator.pop(ctx),
            child: const Text('Got it'),
          ),
        ],
      ),
    );
  }

  String? _platformKey() {
    if (kIsWeb) return null;
    switch (defaultTargetPlatform) {
      case TargetPlatform.android:
        return 'android';
      case TargetPlatform.iOS:
        return 'ios';
      case TargetPlatform.macOS:
        return 'mac';
      case TargetPlatform.windows:
        return 'windows';
      case TargetPlatform.linux:
        return 'linux';
      default:
        return null;
    }
  }

  @override
  void dispose() {
    DeepLinkService.setHandler(onLink: null);
    super.dispose();
  }

  Future<void> _checkOnboarding() async {
    final prefs = await SharedPreferences.getInstance();
    final complete = prefs.getBool('draftright.onboardingComplete') ?? false;
    setState(() => _onboardingComplete = complete);
  }

  Future<void> _completeOnboarding() async {
    // Advance the UI immediately so the user is never trapped on onboarding
    // if the persistence plugin misbehaves (see the iOS plugin-registration
    // bug that made "Get Started" appear unresponsive). Persistence is
    // best-effort afterwards.
    setState(() => _onboardingComplete = true);
    try {
      final prefs = await SharedPreferences.getInstance();
      await prefs.setBool('draftright.onboardingComplete', true);
    } catch (e) {
      DRLogger.warn('Failed to persist onboardingComplete: $e');
    }
  }

  /// Restart the floating bubble if the user had it enabled last session.
  /// No-op on iOS / desktop / web (channel returns false).
  Future<void> _restoreFloatingBubble() async {
    if (!mounted) return;
    final settings = context.read<SettingsService>();
    if (settings.floatingBubbleEnabled && await ShareService.canDrawOverlays()) {
      await ShareService.startBubble();
    }
  }

  /// Drain any deep link the app was launched with and subscribe to
  /// fresh ones while alive.  Today the only event we handle is
  /// [PaymentReturnEvent] (Lemon Squeezy redirect after checkout) —
  /// route to the Subscription screen and let its
  /// `AppLifecycleState.resumed` listener refresh `/subscription`.
  Future<void> _wireDeepLinks() async {
    final initial = await DeepLinkService.getInitialEvent();
    if (initial is PaymentReturnEvent && mounted) {
      _openSubscriptionFromDeepLink(initial);
    }
    DeepLinkService.setHandler(onLink: (event) {
      if (event is PaymentReturnEvent && mounted) {
        _openSubscriptionFromDeepLink(event);
      }
    });
  }

  void _openSubscriptionFromDeepLink(PaymentReturnEvent e) {
    // Defer one frame so the Navigator is mounted on cold-start.
    WidgetsBinding.instance.addPostFrameCallback((_) {
      if (!mounted) return;
      // Use the local Navigator (NOT rootNavigator: true) so the new
      // route lands under DraftRightApp's MultiProvider.  The bootstrap
      // MaterialApp wraps DraftRightApp, so its Navigator is the
      // "root" one — but it lives ABOVE MultiProvider, so routes
      // pushed there can't read AuthService / SettingsService and
      // crash with "Could not find the correct Provider<AuthService>"
      // on SubscriptionScreen's initState.  See bug captured
      // 2026-06-01 on the iPhone 17 Pro simulator.
      final nav = Navigator.of(context);
      // Don't stack duplicate Subscription routes if the user is
      // already on it (e.g. they kicked off checkout, returned via
      // universal link, paid, returned again).
      nav.popUntil((r) => r.isFirst);
      nav.push(MaterialPageRoute(builder: (_) => const SubscriptionScreen()));
      if (!e.success) {
        // User cancelled — surface a quiet hint instead of nothing.
        ScaffoldMessenger.of(context).showSnackBar(const SnackBar(
          content: Text('Checkout cancelled. You can pick a different method.'),
          duration: Duration(seconds: 3),
        ));
      }
    });
  }

  @override
  Widget build(BuildContext context) {
    if (!_onboardingComplete) {
      return OnboardingScreen(onComplete: _completeOnboarding);
    }

    final auth = context.watch<AuthService>();
    if (!auth.isLoggedIn) {
      return const LoginScreen();
    }

    final screens = [const PlaygroundScreen(), const SettingsScreen()];

    return Scaffold(
      body: screens[_currentIndex],
      bottomNavigationBar: NavigationBar(
        selectedIndex: _currentIndex,
        onDestinationSelected: (index) =>
            setState(() => _currentIndex = index),
        destinations: const [
          NavigationDestination(
              icon: Icon(Icons.edit_note), label: 'Playground'),
          NavigationDestination(
              icon: Icon(Icons.settings), label: 'Settings'),
        ],
      ),
    );
  }
}
