import 'package:flutter/material.dart';
import 'package:provider/provider.dart';
import 'package:shared_preferences/shared_preferences.dart';
import 'package:draftright_mobile/services/settings_service.dart';
import 'package:draftright_mobile/screens/onboarding_screen.dart';
import 'package:draftright_mobile/screens/settings_screen.dart';
import 'package:draftright_mobile/screens/playground_screen.dart';

void main() async {
  WidgetsFlutterBinding.ensureInitialized();
  final settings = SettingsService();
  await settings.init();
  runApp(DraftRightApp(settings: settings));
}

class DraftRightApp extends StatelessWidget {
  final SettingsService settings;
  const DraftRightApp({super.key, required this.settings});

  @override
  Widget build(BuildContext context) {
    return ChangeNotifierProvider.value(
      value: settings,
      child: MaterialApp(
        title: 'DraftRight',
        theme: ThemeData(
          colorScheme: ColorScheme.fromSeed(seedColor: Colors.blue),
          useMaterial3: true,
        ),
        darkTheme: ThemeData(
          colorScheme: ColorScheme.fromSeed(seedColor: Colors.blue, brightness: Brightness.dark),
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

    final screens = [const PlaygroundScreen(), const SettingsScreen()];

    return Scaffold(
      body: screens[_currentIndex],
      bottomNavigationBar: NavigationBar(
        selectedIndex: _currentIndex,
        onDestinationSelected: (index) => setState(() => _currentIndex = index),
        destinations: const [
          NavigationDestination(icon: Icon(Icons.edit_note), label: 'Playground'),
          NavigationDestination(icon: Icon(Icons.settings), label: 'Settings'),
        ],
      ),
    );
  }
}
