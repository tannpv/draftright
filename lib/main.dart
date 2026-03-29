import 'dart:io';
import 'package:flutter/material.dart';
import 'package:provider/provider.dart';
import 'package:shared_preferences/shared_preferences.dart';
import 'package:draftright_mobile/services/auth_service.dart';
import 'package:draftright_mobile/services/settings_service.dart';
import 'package:draftright_mobile/screens/login_screen.dart';
import 'package:draftright_mobile/screens/onboarding_screen.dart';
import 'package:draftright_mobile/screens/settings_screen.dart';
import 'package:draftright_mobile/screens/playground_screen.dart';

// Desktop imports — only compiled on desktop platforms
import 'package:draftright_mobile/desktop/desktop_app.dart'
    if (dart.library.html) 'package:draftright_mobile/desktop/desktop_app_stub.dart';

bool get isDesktop =>
    !Platform.isAndroid && !Platform.isIOS && !Platform.isFuchsia;

void main() async {
  WidgetsFlutterBinding.ensureInitialized();
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
