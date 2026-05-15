import '../../models/entity.dart';
import 'bank_catalog.dart';
import 'detector.dart';

class BankAccountDetector implements EntityDetector {
  static final _accountPattern = RegExp(r'\b\d{8,19}\b');

  @override
  List<Entity> detect(String text) {
    final lower = text.toLowerCase();
    final out = <Entity>[];
    for (final m in _accountPattern.allMatches(text)) {
      // Look at +/- 30 chars on the SAME LINE for a bank-name alias.
      final lineStart = _lineStart(text, m.start);
      final lineEnd = _lineEnd(text, m.end);
      final winStart = (m.start - 30).clamp(lineStart, m.start);
      final winEnd = (m.end + 30).clamp(m.end, lineEnd);
      final window = lower.substring(winStart, winEnd);
      String? bank;
      for (final alias in BankCatalog.aliases.keys) {
        if (window.contains(alias)) {
          bank = BankCatalog.aliases[alias];
          break;
        }
      }
      if (bank == null) continue;
      final acct = m.group(0)!;
      out.add(Entity(
        kind: EntityKind.bankAccount,
        value: acct,
        display: '$bank · $acct',
        start: m.start,
        end: m.end,
        source: 'regex',
        confidence: 0.92,
        meta: {'bank': bank},
      ));
    }
    return out;
  }

  static int _lineStart(String text, int idx) {
    final nl = text.lastIndexOf('\n', idx - 1);
    return nl < 0 ? 0 : nl + 1;
  }

  static int _lineEnd(String text, int idx) {
    final nl = text.indexOf('\n', idx);
    return nl < 0 ? text.length : nl;
  }
}
