import 'dart:io' show Platform;
import 'package:flutter/material.dart';

class OnboardingScreen extends StatefulWidget {
  final VoidCallback onComplete;
  const OnboardingScreen({super.key, required this.onComplete});

  @override
  State<OnboardingScreen> createState() => _OnboardingScreenState();
}

class _OnboardingScreenState extends State<OnboardingScreen> {
  final PageController _pageController = PageController();
  int _currentPage = 0;

  @override
  void dispose() {
    _pageController.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final pages = [_buildWelcomePage(), _buildEnableKeyboardPage(), _buildLoginPage()];

    return Scaffold(
      body: SafeArea(
        child: Column(
          children: [
            Expanded(
              child: PageView(
                controller: _pageController,
                onPageChanged: (index) => setState(() => _currentPage = index),
                children: pages,
              ),
            ),
            Padding(
              padding: const EdgeInsets.all(24),
              child: Row(
                mainAxisAlignment: MainAxisAlignment.spaceBetween,
                children: [
                  if (_currentPage > 0)
                    TextButton(
                      onPressed: () => _pageController.previousPage(
                        duration: const Duration(milliseconds: 300), curve: Curves.easeInOut),
                      child: const Text('Back'),
                    )
                  else
                    const SizedBox.shrink(),
                  Row(
                    children: List.generate(pages.length, (i) => Container(
                      margin: const EdgeInsets.symmetric(horizontal: 4),
                      width: 8, height: 8,
                      decoration: BoxDecoration(
                        shape: BoxShape.circle,
                        color: i == _currentPage
                            ? Theme.of(context).colorScheme.primary
                            : Colors.grey.shade300,
                      ),
                    )),
                  ),
                  _currentPage == pages.length - 1
                      ? FilledButton(onPressed: widget.onComplete, child: const Text('Get Started'))
                      : TextButton(
                          onPressed: () => _pageController.nextPage(
                            duration: const Duration(milliseconds: 300), curve: Curves.easeInOut),
                          child: const Text('Next'),
                        ),
                ],
              ),
            ),
          ],
        ),
      ),
    );
  }

  Widget _buildWelcomePage() {
    return const Padding(
      padding: EdgeInsets.all(32),
      child: Column(
        mainAxisAlignment: MainAxisAlignment.center,
        children: [
          Icon(Icons.edit_note, size: 80, color: Colors.blue),
          SizedBox(height: 24),
          Text('DraftRight', style: TextStyle(fontSize: 32, fontWeight: FontWeight.bold)),
          SizedBox(height: 16),
          Text(
            'AI-powered text rewriting right from your keyboard. Rewrite text in any app with one tap.',
            textAlign: TextAlign.center,
            style: TextStyle(fontSize: 16, color: Colors.grey),
          ),
        ],
      ),
    );
  }

  Widget _buildEnableKeyboardPage() {
    final bool isIOS;
    try {
      isIOS = Platform.isIOS;
    } catch (_) {
      // Platform not available in tests
      return const Center(child: Text('Enable keyboard'));
    }

    final steps = isIOS
        ? ['Open Settings app', 'Go to General > Keyboard > Keyboards', 'Tap "Add New Keyboard..."', 'Select "DraftRight"', 'Tap DraftRight > Enable "Allow Full Access"']
        : ['Open Settings app', 'Go to Language & Input > Manage Keyboards', 'Enable "DraftRight"', 'Confirm the permission dialog'];

    return Padding(
      padding: const EdgeInsets.all(32),
      child: Column(
        mainAxisAlignment: MainAxisAlignment.center,
        children: [
          const Icon(Icons.keyboard, size: 60, color: Colors.blue),
          const SizedBox(height: 24),
          const Text('Enable the Keyboard', style: TextStyle(fontSize: 24, fontWeight: FontWeight.bold)),
          const SizedBox(height: 24),
          ...steps.asMap().entries.map((entry) => Padding(
            padding: const EdgeInsets.symmetric(vertical: 6),
            child: Row(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                CircleAvatar(radius: 12, child: Text('${entry.key + 1}', style: const TextStyle(fontSize: 12))),
                const SizedBox(width: 12),
                Expanded(child: Text(entry.value, style: const TextStyle(fontSize: 15))),
              ],
            ),
          )),
          if (isIOS) ...[
            const SizedBox(height: 16),
            const Text('Full Access is required so the keyboard can connect to the AI service.',
              style: TextStyle(fontSize: 13, color: Colors.grey), textAlign: TextAlign.center),
          ],
        ],
      ),
    );
  }

  Widget _buildLoginPage() {
    return const Padding(
      padding: EdgeInsets.all(32),
      child: Column(
        mainAxisAlignment: MainAxisAlignment.center,
        children: [
          Icon(Icons.account_circle_outlined, size: 60, color: Colors.blue),
          SizedBox(height: 24),
          Text('Sign In', style: TextStyle(fontSize: 24, fontWeight: FontWeight.bold)),
          SizedBox(height: 16),
          Text(
            'Create a free DraftRight account or sign in. Your account securely connects the keyboard to the DraftRight AI service.',
            textAlign: TextAlign.center,
            style: TextStyle(fontSize: 16, color: Colors.grey),
          ),
          SizedBox(height: 16),
          Text(
            'After finishing setup, use the Settings tab to sign in or create your account.',
            textAlign: TextAlign.center,
            style: TextStyle(fontSize: 14, color: Colors.grey),
          ),
        ],
      ),
    );
  }
}
