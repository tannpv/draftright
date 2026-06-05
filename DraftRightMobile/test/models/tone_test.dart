import 'package:flutter_test/flutter_test.dart';
import 'package:draftright_mobile/models/tone.dart';

void main() {
  test('Tone.values has 8 tones', () {
    expect(Tone.values.length, 8);
  });

  test('each tone has a non-empty displayName', () {
    for (final tone in Tone.values) {
      expect(tone.displayName.isNotEmpty, true);
    }
  });

  test('each tone has a non-empty systemPrompt', () {
    for (final tone in Tone.values) {
      expect(tone.systemPrompt().isNotEmpty, true);
    }
  });

  test('translate tone includes target language in prompt', () {
    final prompt = Tone.translate.systemPrompt(targetLanguage: 'Vietnamese');
    expect(prompt.contains('Vietnamese'), true);
  });

  test('translate tone defaults to English', () {
    final prompt = Tone.translate.systemPrompt();
    expect(prompt.contains('English'), true);
  });

  test('each tone has an icon name', () {
    for (final tone in Tone.values) {
      expect(tone.iconName.isNotEmpty, true);
    }
  });
}
