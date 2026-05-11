import 'package:flutter/material.dart';

enum Tone {
  simple,
  natural,
  polished,
  concise,
  technical,
  claude,
  grammarCheck,
  translate;

  String get displayName {
    switch (this) {
      case Tone.simple: return 'Simple';
      case Tone.natural: return 'More Natural';
      case Tone.polished: return 'More Polished';
      case Tone.concise: return 'Concise';
      case Tone.technical: return 'Technical';
      case Tone.claude: return 'Claude Style';
      case Tone.grammarCheck: return 'Grammar Check';
      case Tone.translate: return 'Translate';
    }
  }

  String get iconName {
    switch (this) {
      case Tone.simple: return 'text_fields';
      case Tone.natural: return 'chat_bubble_outline';
      case Tone.polished: return 'auto_awesome';
      case Tone.concise: return 'compress';
      case Tone.technical: return 'build';
      case Tone.claude: return 'smart_toy';
      case Tone.grammarCheck: return 'spellcheck';
      case Tone.translate: return 'language';
    }
  }

  IconData get icon {
    switch (this) {
      case Tone.simple: return Icons.text_fields;
      case Tone.natural: return Icons.chat_bubble_outline;
      case Tone.polished: return Icons.auto_awesome;
      case Tone.concise: return Icons.compress;
      case Tone.technical: return Icons.build_outlined;
      case Tone.claude: return Icons.smart_toy;
      case Tone.grammarCheck: return Icons.spellcheck;
      case Tone.translate: return Icons.language;
    }
  }

  String get apiValue {
    switch (this) {
      case Tone.simple: return 'simple';
      case Tone.natural: return 'natural';
      case Tone.polished: return 'polished';
      case Tone.concise: return 'concise';
      case Tone.technical: return 'technical';
      case Tone.claude: return 'claude';
      case Tone.grammarCheck: return 'grammar_check';
      case Tone.translate: return 'translate';
    }
  }

  String systemPrompt({String targetLanguage = 'English'}) {
    switch (this) {
      case Tone.simple:
        return 'Rewrite the following text using simple, easy-to-understand language. Use short sentences and common words. Preserve the original meaning. Return only the rewritten text, no explanations.';
      case Tone.natural:
        return 'Rewrite the following text to sound more natural and conversational, as if spoken by a real person. Remove awkward phrasing and make it flow smoothly. Preserve the original meaning. Return only the rewritten text, no explanations.';
      case Tone.polished:
        return 'Rewrite the following text to be more polished and professional. Improve grammar, word choice, and sentence structure for a refined, workplace-appropriate tone. Preserve the original meaning. Return only the rewritten text, no explanations.';
      case Tone.concise:
        return 'Rewrite the following text to be as concise as possible. Remove unnecessary words, redundancy, and filler while preserving the key meaning. Return only the rewritten text, no explanations.';
      case Tone.technical:
        return 'Rewrite the following text in a technical specification style. Use precise, unambiguous language suitable for documentation, specs, or technical communication. Preserve the original meaning. Return only the rewritten text, no explanations.';
      case Tone.claude:
        return 'Rewrite the following text in a clear, thoughtful, and well-structured style. Be direct but warm — every sentence should carry weight. Use good paragraph breaks and logical flow. Acknowledge nuance where relevant without over-hedging. Sound naturally confident and approachable, not formal or stiff. Preserve the original meaning. Return only the rewritten text, no explanations.';
      case Tone.grammarCheck:
        return 'Analyze the given text for grammar, spelling, and style issues. Return a JSON object with a "score" (0-100) and an "issues" array. Each issue has "type", "offset", "length", "original", "suggestion", and "reason". Return ONLY JSON.';
      case Tone.translate:
        return 'Translate the following text into $targetLanguage. If the text is already in $targetLanguage, translate it into English instead. Preserve the original meaning and tone. Return only the translated text, no explanations.';
    }
  }
}
