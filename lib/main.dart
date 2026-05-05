import 'package:flutter/foundation.dart';
import 'package:flutter/material.dart';
import 'package:provider/provider.dart';
import 'package:shared_preferences/shared_preferences.dart';
import 'package:draftright_mobile/services/auth_service.dart';
import 'package:draftright_mobile/services/logger_service.dart';
import 'package:draftright_mobile/services/settings_service.dart';
import 'package:draftright_mobile/screens/login_screen.dart';
import 'package:draftright_mobile/screens/onboarding_screen.dart';
import 'package:draftright_mobile/screens/settings_screen.dart';
import 'package:draftright_mobile/screens/playground_screen.dart';
import 'package:draftright_mobile/screens/share_rewrite_screen.dart';
import 'package:draftright_mobile/services/share_service.dart';

// Desktop imports — only compiled on desktop platforms
import 'package:draftright_mobile/desktop/desktop_app.dart'
    if (dart.library.html) 'package:draftright_mobile/desktop/desktop_app_stub.dart';

bool get isDesktop =>
    !kIsWeb &&
    (defaultTargetPlatform == TargetPlatform.windows ||
     defaultTargetPlatform == TargetPlatform.linux ||
     defaultTargetPlatform == TargetPlatform.macOS);

void main() async {
  WidgetsFlutterBinding.ensureInitialized();
  await DRLogger.init();
  DRLogger.log('App started', category: 'APP');
  final settings = SettingsService();
  await settings.init();

  final auth = AuthService();
  await auth.init(settings.backendUrl);

  if (isDesktop) {
    // Windows / Linux: run as system tray app with floating panel
    runApp(DesktopApp(settings: settings, auth: auth));
  } else {
    // Android / iOS: run as mobile app
    runApp(DraftRightApp(settings: settings, auth: auth));
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
    _wireShareIntake();
  }

  @override
  void dispose() {
    ShareService.setHandler();
    super.dispose();
  }

  Future<void> _checkOnboarding() async {
    final prefs = await SharedPreferences.getInstance();
    final complete = prefs.getBool('draftright.onboardingComplete') ?? false;
    setState(() => _onboardingComplete = complete);
  }

  Future<void> _completeOnboarding() async {
    final prefs = await SharedPreferences.getInstance();
    await prefs.setBool('draftright.onboardingComplete', true);
    setState(() => _onboardingComplete = true);
  }

  /// Drain any text the user shared on cold-start, and subscribe to fresh
  /// shares while the app is alive.  Routes straight to ShareRewriteScreen
  /// (skipping the bottom-nav Playground) so the user gets a fast tone
  /// picker for the exact text they shared.
  Future<void> _wireShareIntake() async {
    final initial = await ShareService.getInitialSharedText();
    if (initial != null && mounted) _openShareRewrite(initial);
    ShareService.setHandler(
      onSharedText: (text) {
        if (mounted) _openShareRewrite(text);
      },
      onBubbleEmptyClipboard: () {
        if (!mounted) return;
        ScaffoldMessenger.of(context).showSnackBar(const SnackBar(
          content: Text('Clipboard is empty. Copy text first, then tap the bubble.'),
          duration: Duration(seconds: 3),
        ));
      },
    );
    // Restart floating bubble if the user had it enabled last session.
    // No-op on iOS / desktop / web (channel returns false).
    if (!mounted) return;
    final settings = context.read<SettingsService>();
    if (settings.floatingBubbleEnabled && await ShareService.canDrawOverlays()) {
      await ShareService.startBubble();
    }
  }

  void _openShareRewrite(String text) {
    // Defer one frame so the Navigator is mounted on cold-start.
    WidgetsBinding.instance.addPostFrameCallback((_) {
      if (!mounted) return;
      final nav = Navigator.of(context, rootNavigator: true);
      // Avoid stacking duplicate share screens.
      nav.popUntil((r) => r.isFirst);
      nav.push(MaterialPageRoute(
        builder: (_) => ShareRewriteScreen(sharedText: text),
      ));
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
