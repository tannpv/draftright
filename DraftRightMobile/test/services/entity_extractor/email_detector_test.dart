import 'package:flutter_test/flutter_test.dart';
import 'package:draftright_mobile/services/entity_extractor/email_detector.dart';
import 'package:draftright_mobile/models/entity.dart';

void main() {
  final det = EmailDetector();
  test('basic email', () {
    final r = det.detect('Contact tan@gmail.com please');
    expect(r.single.kind, EntityKind.email);
    expect(r.single.value, 'tan@gmail.com');
    expect(r.single.display, 'tan@gmail.com');
  });

  test('email with subdomain + plus tag', () {
    final r = det.detect('Mail to tan.foo+bar@sub.example.co.uk thanks');
    expect(r.single.value, 'tan.foo+bar@sub.example.co.uk');
  });

  test('strips trailing punctuation', () {
    final r = det.detect('email tan@x.com, or call');
    expect(r.single.value, 'tan@x.com');
  });

  test('rejects malformed', () {
    expect(det.detect('tan@@x.com'), isEmpty);
    expect(det.detect('@x.com'), isEmpty);
  });
}
