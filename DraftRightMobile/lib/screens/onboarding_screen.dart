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
    final pages = [
      _buildWelcomePage(),
      _buildProcessTextPage(),
      _buildOptionalKeyboardPage(),
      _buildLoginPage(),
    ];

    return Scaffold(
      body: SafeArea(
        child: Column(
          children: [
            Expanded(
              child: PageView(
                controller: _pageController,
                onPageChanged: (index) => setState(() => _currentPage = index),
                children: pages.map(_scrollableCentered).toList(),
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

  /// Lets a fixed-height, vertically-centered page scroll instead of
  /// overflowing on short screens: the content stays centered when it fits the
  /// viewport and becomes scrollable when it doesn't.
  Widget _scrollableCentered(Widget page) {
    return LayoutBuilder(
      builder: (context, constraints) => SingleChildScrollView(
        child: ConstrainedBox(
          constraints: BoxConstraints(minHeight: constraints.maxHeight),
          child: IntrinsicHeight(child: page),
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
            'Rewrite text in any app, in any language. Works alongside the keyboard you already use.',
            textAlign: TextAlign.center,
            style: TextStyle(fontSize: 16, color: Colors.grey),
          ),
        ],
      ),
    );
  }

  /// PRIMARY flow on Android: Process Text long-press menu.
  /// Works with Gboard / Samsung Keyboard / any IME, in any language.
  Widget _buildProcessTextPage() {
    const steps = [
      _StepLine(num: 1, text: 'Type with the keyboard you already use'),
      _StepLine(num: 2, text: 'Long-press the sentence you want to rewrite'),
      _StepLine(num: 3, text: 'Tap "DraftRight" in the popup menu'),
      _StepLine(num: 4, text: 'Pick a tone — text is rewritten in place'),
    ];

    return Padding(
      padding: const EdgeInsets.all(32),
      child: Column(
        mainAxisAlignment: MainAxisAlignment.center,
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          const Center(child: Icon(Icons.text_fields, size: 60, color: Colors.blue)),
          const SizedBox(height: 20),
          const Center(
            child: Text('Use in Any App',
                style: TextStyle(fontSize: 24, fontWeight: FontWeight.bold)),
          ),
          const SizedBox(height: 8),
          const Center(
            child: Text('Recommended for everyone',
                style: TextStyle(fontSize: 13, color: Colors.green, fontWeight: FontWeight.w600)),
          ),
          const SizedBox(height: 24),
          ...steps,
          const SizedBox(height: 20),
          Container(
            padding: const EdgeInsets.all(12),
            decoration: BoxDecoration(
              color: Colors.green.shade50,
              borderRadius: BorderRadius.circular(8),
            ),
            child: const Text(
              'Keep your phone\'s keyboard. Vietnamese, Chinese, Japanese — they all work, because typing stays with the keyboard you already love.',
              style: TextStyle(fontSize: 13, color: Colors.black87),
            ),
          ),
        ],
      ),
    );
  }

  /// OPTIONAL flow: install the DraftRight keyboard for one-tap rewriting
  /// while typing. Demoted from primary because it can't handle non-Latin
  /// language input methods (Telex, Pinyin, Hangul, etc.).
  Widget _buildOptionalKeyboardPage() {
    final bool isIOS;
    try {
      isIOS = Platform.isIOS;
    } catch (_) {
      return const Center(child: Text('Optional: install keyboard'));
    }

    final steps = isIOS
        ? const ['Open Settings', 'General > Keyboard > Keyboards', 'Add New Keyboard > DraftRight', 'Tap DraftRight > Allow Full Access']
        : const ['Open Settings', 'Language & Input > Manage Keyboards', 'Enable "DraftRight"', 'Confirm the permission dialog'];

    return Padding(
      padding: const EdgeInsets.all(32),
      child: Column(
        mainAxisAlignment: MainAxisAlignment.center,
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          const Center(child: Icon(Icons.keyboard, size: 56, color: Colors.grey)),
          const SizedBox(height: 16),
          const Center(
            child: Text('DraftRight Keyboard',
                style: TextStyle(fontSize: 22, fontWeight: FontWeight.bold)),
          ),
          const SizedBox(height: 4),
          const Center(
            child: Text('Optional — power users only',
                style: TextStyle(fontSize: 13, color: Colors.orange, fontWeight: FontWeight.w600)),
          ),
          const SizedBox(height: 16),
          const Text(
            'A keyboard with built-in tone buttons, so you can rewrite without leaving the typing flow.',
            style: TextStyle(fontSize: 14, color: Colors.black87),
          ),
          const SizedBox(height: 16),
          Container(
            padding: const EdgeInsets.all(12),
            decoration: BoxDecoration(
              color: Colors.orange.shade50,
              borderRadius: BorderRadius.circular(8),
              border: Border.all(color: Colors.orange.shade200),
            ),
            child: const Row(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Icon(Icons.info_outline, size: 18, color: Colors.orange),
                SizedBox(width: 8),
                Expanded(
                  child: Text(
                    'Now types English, Tiếng Việt, Français, Español, Deutsch, Italiano, and Português. Tap the globe key to switch. For Chinese, Japanese, Korean, Thai — use your phone\'s keyboard with the long-press flow on the previous page.',
                    style: TextStyle(fontSize: 12, color: Colors.black87),
                  ),
                ),
              ],
            ),
          ),
          const SizedBox(height: 16),
          const Text('To enable later:', style: TextStyle(fontSize: 13, fontWeight: FontWeight.w600)),
          const SizedBox(height: 8),
          ...steps.asMap().entries.map((entry) => Padding(
            padding: const EdgeInsets.symmetric(vertical: 3),
            child: Row(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                CircleAvatar(radius: 10, backgroundColor: Colors.grey.shade300,
                    child: Text('${entry.key + 1}',
                        style: const TextStyle(fontSize: 10, color: Colors.black87))),
                const SizedBox(width: 10),
                Expanded(child: Text(entry.value, style: const TextStyle(fontSize: 13))),
              ],
            ),
          )),
          if (isIOS) ...[
            const SizedBox(height: 8),
            const Text('Full Access lets the keyboard reach the AI service.',
                style: TextStyle(fontSize: 11, color: Colors.grey)),
          ],
          const SizedBox(height: 12),
          const Center(
            child: Text('You can skip this — long-press works without it.',
                style: TextStyle(fontSize: 12, color: Colors.grey, fontStyle: FontStyle.italic)),
          ),
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
            'Create a free DraftRight account or sign in. Your account connects DraftRight to the AI service securely.',
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

class _StepLine extends StatelessWidget {
  final int num;
  final String text;
  const _StepLine({required this.num, required this.text});

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.symmetric(vertical: 6),
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          CircleAvatar(
            radius: 12,
            backgroundColor: Colors.blue,
            child: Text('$num',
                style: const TextStyle(fontSize: 12, color: Colors.white, fontWeight: FontWeight.w600)),
          ),
          const SizedBox(width: 12),
          Expanded(child: Text(text, style: const TextStyle(fontSize: 15))),
        ],
      ),
    );
  }
}
