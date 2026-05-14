# Entity Extraction Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add automatic extraction of phone/email/URL/OTP/credit-card (regex, client-side) + address/name/date/bank-account (LLM, on-demand) from shared chat messages in DraftRight Mobile (Flutter), with one-tap copy per entity.

**Architecture:** Flutter `EntityExtractor` (pure Dart regex catalog) runs synchronously when `ShareRewriteScreen` receives shared text. If entities found, navigate to `EntitySheetScreen` (grouped-list UI) instead of tone picker. A `✨ Smart scan` button calls `POST /extract` on the NestJS backend, which builds a strict-JSON extraction prompt, calls the user's AI provider, validates output, and returns merged entities. All errors degrade gracefully — regex results never disappear.

**Tech Stack:** Flutter 3.x / Dart, NestJS 10, TypeORM (no DB writes in this feature), existing `AiProvidersService`, JWT auth via existing `JwtAuthGuard`, `class-validator` DTOs.

**Spec:** `docs/superpowers/specs/2026-05-14-entity-extraction-design.md`

---

## File Structure

### Flutter (new)
```
DraftRightMobile/
  lib/
    models/
      entity.dart                              # Entity data class + EntityKind enum
    services/
      entity_extractor.dart                    # Pure Dart regex engine
      entity_extractor/
        detector.dart                          # EntityDetector interface
        phone_detector.dart
        email_detector.dart
        url_detector.dart
        otp_detector.dart
        credit_card_detector.dart
        bank_account_detector.dart
        bank_catalog.dart                      # VN bank names lookup
      extraction_api.dart                      # POST /extract client
    screens/
      entity_sheet_screen.dart                 # Grouped-list UI
  test/
    services/
      entity_extractor_test.dart
      extraction_api_test.dart
    screens/
      entity_sheet_screen_test.dart
```

### Flutter (modified)
```
DraftRightMobile/
  lib/
    screens/
      share_rewrite_screen.dart                # Branch into entity sheet when entities present
```

### Backend (new)
```
backend/
  src/
    extraction/
      extraction.module.ts
      extraction.controller.ts
      extraction.service.ts
      dto/
        extract.dto.ts
  test/
    extraction/
      extraction.service.spec.ts
      extraction.controller.e2e-spec.ts
```

### Backend (modified)
```
backend/
  src/
    app.module.ts                              # Register ExtractionModule
```

### Docs (modified)
```
docs/
  test-cases.xlsx                              # Add EXTRACT-001..010
```

---

## Phase 0 — Prep

### Task 0.1: Add test case IDs to docs/test-cases.xlsx

**Files:**
- Modify: `docs/test-cases.xlsx`

- [ ] **Step 1:** Open `docs/test-cases.xlsx`. Add the following rows on the appropriate sheet (use the existing format — typically columns: ID, Feature, Description, Steps, Expected, Platform).

```
EXTRACT-001 | Entity Extraction | Share message containing phone+email → entity sheet shows 2 rows; tap copy on each works | 1. From any chat, long-press a msg with "Call 0912345678 or email tan@x.com". 2. Share → DraftRight. 3. Tap copy on phone. 4. Tap copy on email. | Entity sheet renders 2 group cards; SnackBar "Phone copied" / "Email copied"; Clipboard contains correct value. | Android + iOS
EXTRACT-002 | Entity Extraction | Share message with no entities → tone picker shows (regression check) | 1. Share prose "Hello, hope you are well." 2. Observe screen. | Tone picker renders; entity sheet never mounts. | Android + iOS
EXTRACT-003 | Entity Extraction | Smart scan adds Vietnamese address "123 Lê Lợi Q1" | 1. Share msg containing "Địa chỉ 123 Lê Lợi Q1". 2. Tap ✨ Smart scan. | After spinner: a 🏠 Address row appears with display "123 Lê Lợi Q1"; button hides. | Android + iOS
EXTRACT-004 | Entity Extraction | Smart scan offline → snackbar, regex results intact | 1. Airplane mode. 2. Share msg with phone+address. 3. Tap ✨ Smart scan. | SnackBar "Smart scan unavailable"; phone row still visible & copyable; Smart scan button re-enabled. | Android + iOS
EXTRACT-005 | Entity Extraction | Card number masked on render, full value copied | 1. Share msg "Card 4242 4242 4242 4242". 2. Observe display. 3. Long-press to reveal. 4. Tap copy. | Display shows "**** **** **** 4242"; long-press reveals full; copy yields "4242424242424242". | Android + iOS
EXTRACT-006 | Entity Extraction | OTP detected only when trigger word present | 1. Share msg "Năm 2024 là năm tốt." 2. Share msg "OTP 482917". | Msg #1: empty (no trigger). Msg #2: 🔢 OTP row with 482917. | Android + iOS
EXTRACT-007 | Entity Extraction | Long message (5000 chars) → regex < 50ms, no UI freeze | 1. Share a 5000-char text containing 10 phones + 5 emails. 2. Observe load. | Sheet renders within 100ms; no jank; all 15 rows visible. | Android + iOS
EXTRACT-008 | Entity Extraction | iOS share extension same flow | 1. On iOS, share msg from Notes / Messages app via share sheet. 2. Pick DraftRight. | Same entity sheet renders as Android. | iOS
EXTRACT-009 | Entity Extraction | Quota exceeded → 402 snackbar, regex results stay | 1. Exhaust free-tier quota. 2. Share msg with phone+address. 3. Tap ✨ Smart scan. | SnackBar "Smart scan limit reached"; phone row still visible. | Android + iOS
EXTRACT-010 | Entity Extraction | Vietnamese diacritics + emoji in source → offsets correct | 1. Share msg "Gọi 0912345678 nhé 😊". 2. Verify entity selection highlights. | Phone row shows; copy yields "+84912345678"; no offset crash; UTF-16 surrogates handled. | Android + iOS
```

- [ ] **Step 2:** Save the xlsx. Commit.

```bash
git checkout develop
git pull
git checkout -b feature/entity-extraction-20260514
git add docs/test-cases.xlsx
git commit -m "test(extraction): add test cases EXTRACT-001..010"
```

---

## Phase 1 — Dart data model

### Task 1.1: Create `Entity` model and `EntityKind` enum

**Files:**
- Create: `DraftRightMobile/lib/models/entity.dart`
- Create: `DraftRightMobile/test/models/entity_test.dart`

- [ ] **Step 1: Write failing test**

```dart
// DraftRightMobile/test/models/entity_test.dart
import 'package:flutter_test/flutter_test.dart';
import 'package:draftright_mobile/models/entity.dart';

void main() {
  group('Entity', () {
    test('equality uses kind + value (case-insensitive)', () {
      final a = Entity(
        kind: EntityKind.email,
        value: 'TAN@X.COM',
        display: 'tan@x.com',
        start: 0,
        end: 9,
        source: 'regex',
        confidence: 1.0,
      );
      final b = Entity(
        kind: EntityKind.email,
        value: 'tan@x.com',
        display: 'TAN@X.COM',
        start: 100,
        end: 109,
        source: 'llm',
        confidence: 0.7,
      );
      expect(a.dedupeKey, b.dedupeKey);
    });

    test('toJson / fromJson round trip', () {
      final e = Entity(
        kind: EntityKind.bankAccount,
        value: '0123456789',
        display: 'Vietcombank · 0123456789',
        start: 10,
        end: 20,
        source: 'regex',
        confidence: 0.95,
        meta: const {'bank': 'Vietcombank'},
      );
      final json = e.toJson();
      final round = Entity.fromJson(json);
      expect(round.kind, EntityKind.bankAccount);
      expect(round.value, '0123456789');
      expect(round.meta['bank'], 'Vietcombank');
    });
  });
}
```

- [ ] **Step 2: Run test, expect failure**

```bash
cd DraftRightMobile && flutter test test/models/entity_test.dart
```
Expected: FAIL — `entity.dart` not found.

- [ ] **Step 3: Implement model**

```dart
// DraftRightMobile/lib/models/entity.dart
enum EntityKind {
  phone,
  email,
  url,
  otp,
  creditCard,
  address,
  personName,
  dateTime,
  bankAccount,
}

extension EntityKindCodec on EntityKind {
  String get wireName => switch (this) {
    EntityKind.phone => 'phone',
    EntityKind.email => 'email',
    EntityKind.url => 'url',
    EntityKind.otp => 'otp',
    EntityKind.creditCard => 'creditCard',
    EntityKind.address => 'address',
    EntityKind.personName => 'personName',
    EntityKind.dateTime => 'dateTime',
    EntityKind.bankAccount => 'bankAccount',
  };

  static EntityKind fromWire(String s) =>
      EntityKind.values.firstWhere((k) => k.wireName == s);
}

class Entity {
  final EntityKind kind;
  final String value;
  final String display;
  final int start;
  final int end;
  final String source;       // "regex" | "llm"
  final double confidence;
  final Map<String, String> meta;

  const Entity({
    required this.kind,
    required this.value,
    required this.display,
    required this.start,
    required this.end,
    required this.source,
    required this.confidence,
    this.meta = const {},
  });

  String get dedupeKey => '${kind.wireName}:${value.toLowerCase()}';

  Map<String, dynamic> toJson() => {
    'kind': kind.wireName,
    'value': value,
    'display': display,
    'start': start,
    'end': end,
    'source': source,
    'confidence': confidence,
    'meta': meta,
  };

  factory Entity.fromJson(Map<String, dynamic> json) => Entity(
    kind: EntityKindCodec.fromWire(json['kind'] as String),
    value: json['value'] as String,
    display: json['display'] as String,
    start: json['start'] as int,
    end: json['end'] as int,
    source: json['source'] as String? ?? 'llm',
    confidence: (json['confidence'] as num?)?.toDouble() ?? 0.5,
    meta: (json['meta'] as Map?)?.map(
          (k, v) => MapEntry(k.toString(), v.toString()),
        ) ??
        const {},
  );
}
```

- [ ] **Step 4: Run test, expect pass**

```bash
cd DraftRightMobile && flutter test test/models/entity_test.dart
```
Expected: PASS — both tests green.

- [ ] **Step 5: Commit**

```bash
git add DraftRightMobile/lib/models/entity.dart DraftRightMobile/test/models/entity_test.dart
git commit -m "feat(extraction): add Entity model + EntityKind enum"
```

---

## Phase 2 — Dart EntityExtractor (TDD per detector)

### Task 2.1: Create `EntityDetector` interface + `EntityExtractor` shell

**Files:**
- Create: `DraftRightMobile/lib/services/entity_extractor/detector.dart`
- Create: `DraftRightMobile/lib/services/entity_extractor.dart`
- Create: `DraftRightMobile/test/services/entity_extractor_test.dart`

- [ ] **Step 1: Write failing test for shell**

```dart
// DraftRightMobile/test/services/entity_extractor_test.dart
import 'package:flutter_test/flutter_test.dart';
import 'package:draftright_mobile/services/entity_extractor.dart';
import 'package:draftright_mobile/models/entity.dart';

void main() {
  group('EntityExtractor — shell', () {
    test('empty input → empty list', () {
      expect(EntityExtractor.extract(''), isEmpty);
    });

    test('whitespace only → empty list', () {
      expect(EntityExtractor.extract('   \n  \t'), isEmpty);
    });

    test('dedupe collapses (kind, value) duplicates', () {
      // Two phones in different formats but same E.164 normalized value
      // should collapse to one. Will be filled in by phone detector task.
      final out = EntityExtractor.extract('Call 0912345678 or +84912345678');
      final phones = out.where((e) => e.kind == EntityKind.phone).toList();
      expect(phones.length, 1, reason: 'normalized phones must dedupe');
    });
  });
}
```

- [ ] **Step 2: Run, expect compile failure**

```bash
cd DraftRightMobile && flutter test test/services/entity_extractor_test.dart
```
Expected: FAIL — `entity_extractor.dart` not found.

- [ ] **Step 3: Implement shell**

```dart
// DraftRightMobile/lib/services/entity_extractor/detector.dart
import '../../models/entity.dart';

abstract class EntityDetector {
  List<Entity> detect(String text);
}
```

```dart
// DraftRightMobile/lib/services/entity_extractor.dart
import '../models/entity.dart';
import 'entity_extractor/detector.dart';

class EntityExtractor {
  static final List<EntityDetector> _detectors = <EntityDetector>[
    // Detectors registered in subsequent tasks. Order does not matter —
    // dedupe is applied after collecting from all detectors.
  ];

  /// Pure-function entry. Runs every detector, dedupes by (kind, value)
  /// case-insensitive, returns entities sorted by start offset.
  static List<Entity> extract(String text) {
    if (text.trim().isEmpty) return const [];
    final all = <Entity>[];
    for (final d in _detectors) {
      all.addAll(d.detect(text));
    }
    return _dedupe(all)..sort((a, b) => a.start.compareTo(b.start));
  }

  static List<Entity> _dedupe(List<Entity> input) {
    final byKey = <String, Entity>{};
    for (final e in input) {
      final existing = byKey[e.dedupeKey];
      if (existing == null) {
        byKey[e.dedupeKey] = e;
      } else {
        // Higher confidence wins; ties: regex over llm.
        final keepNew = e.confidence > existing.confidence ||
            (e.confidence == existing.confidence &&
                e.source == 'regex' &&
                existing.source != 'regex');
        if (keepNew) byKey[e.dedupeKey] = e;
      }
    }
    return byKey.values.toList();
  }
}
```

- [ ] **Step 4: Run, first two tests pass; third fails (no phone detector yet)**

```bash
cd DraftRightMobile && flutter test test/services/entity_extractor_test.dart
```
Expected: 2 PASS, 1 FAIL (dedupe test — no phones detected yet). Acceptable; phone detector task fixes it.

- [ ] **Step 5: Commit**

```bash
git add DraftRightMobile/lib/services/entity_extractor.dart \
        DraftRightMobile/lib/services/entity_extractor/detector.dart \
        DraftRightMobile/test/services/entity_extractor_test.dart
git commit -m "feat(extraction): EntityExtractor shell + EntityDetector interface"
```

---

### Task 2.2: Phone detector

**Files:**
- Create: `DraftRightMobile/lib/services/entity_extractor/phone_detector.dart`
- Modify: `DraftRightMobile/lib/services/entity_extractor.dart`
- Create/append: `DraftRightMobile/test/services/entity_extractor/phone_detector_test.dart`

- [ ] **Step 1: Write failing tests**

```dart
// DraftRightMobile/test/services/entity_extractor/phone_detector_test.dart
import 'package:flutter_test/flutter_test.dart';
import 'package:draftright_mobile/services/entity_extractor/phone_detector.dart';
import 'package:draftright_mobile/models/entity.dart';

void main() {
  final det = PhoneDetector();
  test('VN local 0xx normalizes to +84xx', () {
    final r = det.detect('Gọi 0912 345 678 nhé');
    expect(r, hasLength(1));
    expect(r.single.kind, EntityKind.phone);
    expect(r.single.value, '+84912345678');
    expect(r.single.display, '0912 345 678');
  });

  test('VN +84 international form', () {
    final r = det.detect('Hotline +84 912 345 678');
    expect(r.single.value, '+84912345678');
  });

  test('US international form +1 (415) 555-2671', () {
    final r = det.detect('US +1 (415) 555-2671');
    expect(r.single.value, '+14155552671');
  });

  test('rejects short numbers and 4-digit years', () {
    expect(det.detect('năm 2024 trời ơi'), isEmpty);
    expect(det.detect('code 1234'), isEmpty);
  });

  test('offsets round trip — text.substring(start, end) matches', () {
    final src = 'A phone 0912345678 here';
    final r = det.detect(src).single;
    expect(src.substring(r.start, r.end), '0912345678');
  });
}
```

- [ ] **Step 2: Run, expect failure**

```bash
cd DraftRightMobile && flutter test test/services/entity_extractor/phone_detector_test.dart
```
Expected: FAIL — file missing.

- [ ] **Step 3: Implement**

```dart
// DraftRightMobile/lib/services/entity_extractor/phone_detector.dart
import '../../models/entity.dart';
import 'detector.dart';

class PhoneDetector implements EntityDetector {
  // VN local: 0 followed by 9-10 digits (with optional spaces/dashes between)
  // VN intl:  +84 followed by 9-10 digits
  // Generic intl: +<country 1-3 digits> <body 6-13 digits>
  static final _patterns = <RegExp>[
    // +country then digits; allow spaces, dashes, parens
    RegExp(r'\+\d{1,3}[\s\-]?\(?\d{1,4}\)?[\s\-]?\d{3,4}[\s\-]?\d{3,4}\b'),
    // VN local 0 prefix
    RegExp(r'(?<![\d+])0\d{2}[\s\-]?\d{3}[\s\-]?\d{3,4}\b'),
  ];

  @override
  List<Entity> detect(String text) {
    final out = <Entity>[];
    final seenStarts = <int>{};
    for (final p in _patterns) {
      for (final m in p.allMatches(text)) {
        if (seenStarts.contains(m.start)) continue;
        seenStarts.add(m.start);
        final raw = m.group(0)!;
        final digits = raw.replaceAll(RegExp(r'[\s\-\(\)]'), '');
        final normalized = _toE164(digits);
        if (normalized == null) continue;
        out.add(Entity(
          kind: EntityKind.phone,
          value: normalized,
          display: raw.trim(),
          start: m.start,
          end: m.end,
          source: 'regex',
          confidence: 0.95,
        ));
      }
    }
    return out;
  }

  String? _toE164(String digits) {
    if (digits.startsWith('+')) {
      // Already international; ensure length sane.
      if (digits.length < 8 || digits.length > 16) return null;
      return digits;
    }
    if (digits.startsWith('0') && digits.length >= 10 && digits.length <= 11) {
      return '+84${digits.substring(1)}';
    }
    return null;
  }
}
```

Register in `EntityExtractor._detectors`:

```dart
// DraftRightMobile/lib/services/entity_extractor.dart  (modify _detectors)
import 'entity_extractor/phone_detector.dart';
// ...
  static final List<EntityDetector> _detectors = <EntityDetector>[
    PhoneDetector(),
  ];
```

- [ ] **Step 4: Run all extractor tests, expect pass**

```bash
cd DraftRightMobile && flutter test test/services/entity_extractor_test.dart test/services/entity_extractor/phone_detector_test.dart
```
Expected: PASS for both files. The dedupe test in `entity_extractor_test.dart` should now pass since two phone formats normalize identically.

- [ ] **Step 5: Commit**

```bash
git add DraftRightMobile/lib/services/entity_extractor/phone_detector.dart \
        DraftRightMobile/lib/services/entity_extractor.dart \
        DraftRightMobile/test/services/entity_extractor/phone_detector_test.dart
git commit -m "feat(extraction): phone detector — VN local + international E.164"
```

---

### Task 2.3: Email detector

**Files:**
- Create: `DraftRightMobile/lib/services/entity_extractor/email_detector.dart`
- Modify: `DraftRightMobile/lib/services/entity_extractor.dart`
- Create: `DraftRightMobile/test/services/entity_extractor/email_detector_test.dart`

- [ ] **Step 1: Write failing tests**

```dart
// DraftRightMobile/test/services/entity_extractor/email_detector_test.dart
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
```

- [ ] **Step 2: Run, expect failure**

```bash
cd DraftRightMobile && flutter test test/services/entity_extractor/email_detector_test.dart
```
Expected: FAIL — file missing.

- [ ] **Step 3: Implement**

```dart
// DraftRightMobile/lib/services/entity_extractor/email_detector.dart
import '../../models/entity.dart';
import 'detector.dart';

class EmailDetector implements EntityDetector {
  // Standard pragmatic email regex. Local part allows letters/digits/dot/_/%/+/-.
  static final _pattern =
      RegExp(r'\b[A-Za-z0-9._%+\-]+@[A-Za-z0-9.\-]+\.[A-Za-z]{2,}\b');

  @override
  List<Entity> detect(String text) {
    final out = <Entity>[];
    for (final m in _pattern.allMatches(text)) {
      final raw = m.group(0)!;
      // Reject double-@.
      if ('@'.allMatches(raw).length != 1) continue;
      out.add(Entity(
        kind: EntityKind.email,
        value: raw.toLowerCase(),
        display: raw,
        start: m.start,
        end: m.end,
        source: 'regex',
        confidence: 0.98,
      ));
    }
    return out;
  }
}
```

Register:

```dart
// DraftRightMobile/lib/services/entity_extractor.dart
import 'entity_extractor/email_detector.dart';
// ...
  static final List<EntityDetector> _detectors = <EntityDetector>[
    PhoneDetector(),
    EmailDetector(),
  ];
```

- [ ] **Step 4: Run, expect pass**

```bash
cd DraftRightMobile && flutter test test/services/entity_extractor/email_detector_test.dart
```
Expected: PASS — all four tests green.

- [ ] **Step 5: Commit**

```bash
git add DraftRightMobile/lib/services/entity_extractor/email_detector.dart \
        DraftRightMobile/lib/services/entity_extractor.dart \
        DraftRightMobile/test/services/entity_extractor/email_detector_test.dart
git commit -m "feat(extraction): email detector"
```

---

### Task 2.4: URL detector

**Files:**
- Create: `DraftRightMobile/lib/services/entity_extractor/url_detector.dart`
- Modify: `DraftRightMobile/lib/services/entity_extractor.dart`
- Create: `DraftRightMobile/test/services/entity_extractor/url_detector_test.dart`

- [ ] **Step 1: Failing test**

```dart
// DraftRightMobile/test/services/entity_extractor/url_detector_test.dart
import 'package:flutter_test/flutter_test.dart';
import 'package:draftright_mobile/services/entity_extractor/url_detector.dart';
import 'package:draftright_mobile/models/entity.dart';

void main() {
  final det = UrlDetector();

  test('https with path', () {
    final r = det.detect('Visit https://shop.com/item?id=42 now');
    expect(r.single.kind, EntityKind.url);
    expect(r.single.value, 'https://shop.com/item?id=42');
  });

  test('bare domain with TLD allowed', () {
    final r = det.detect('Web shop.com or foo.vn');
    expect(r.length, 2);
    expect(r.map((e) => e.value), containsAll(['shop.com', 'foo.vn']));
  });

  test('strips trailing punctuation', () {
    final r = det.detect('Go to https://x.com/y. Cool, right?');
    expect(r.single.value, 'https://x.com/y');
  });

  test('rejects bare hello.world (not a real TLD)', () {
    expect(det.detect('hello.world'), isEmpty);
  });
}
```

- [ ] **Step 2: Run, expect FAIL**

```bash
cd DraftRightMobile && flutter test test/services/entity_extractor/url_detector_test.dart
```

- [ ] **Step 3: Implement**

```dart
// DraftRightMobile/lib/services/entity_extractor/url_detector.dart
import '../../models/entity.dart';
import 'detector.dart';

class UrlDetector implements EntityDetector {
  static const _tlds = <String>{
    'com', 'vn', 'net', 'org', 'io', 'co', 'app', 'me', 'info',
    'biz', 'asia', 'tv', 'gg', 'ai', 'dev', 'xyz',
  };

  static final _httpPattern = RegExp(r'https?://\S+', caseSensitive: false);
  static final _barePattern = RegExp(
    r'\b(?:[a-z0-9\-]+\.)+([a-z]{2,6})(?:/\S*)?\b',
    caseSensitive: false,
  );

  @override
  List<Entity> detect(String text) {
    final out = <Entity>[];
    final consumed = <int>{};
    for (final m in _httpPattern.allMatches(text)) {
      final raw = _stripTrailingPunct(m.group(0)!);
      out.add(Entity(
        kind: EntityKind.url,
        value: raw,
        display: raw,
        start: m.start,
        end: m.start + raw.length,
        source: 'regex',
        confidence: 0.98,
      ));
      for (var i = m.start; i < m.start + raw.length; i++) {
        consumed.add(i);
      }
    }
    for (final m in _barePattern.allMatches(text)) {
      if (consumed.contains(m.start)) continue;
      final tld = m.group(1)!.toLowerCase();
      if (!_tlds.contains(tld)) continue;
      final raw = _stripTrailingPunct(m.group(0)!);
      out.add(Entity(
        kind: EntityKind.url,
        value: raw,
        display: raw,
        start: m.start,
        end: m.start + raw.length,
        source: 'regex',
        confidence: 0.85,
      ));
    }
    return out;
  }

  String _stripTrailingPunct(String s) {
    var end = s.length;
    while (end > 0 && '.,!?;:)'.contains(s[end - 1])) {
      end--;
    }
    return s.substring(0, end);
  }
}
```

Register:

```dart
import 'entity_extractor/url_detector.dart';
// ...
  static final List<EntityDetector> _detectors = <EntityDetector>[
    PhoneDetector(),
    EmailDetector(),
    UrlDetector(),
  ];
```

- [ ] **Step 4: Run, expect PASS**

```bash
cd DraftRightMobile && flutter test test/services/entity_extractor/url_detector_test.dart
```

- [ ] **Step 5: Commit**

```bash
git add DraftRightMobile/lib/services/entity_extractor/url_detector.dart \
        DraftRightMobile/lib/services/entity_extractor.dart \
        DraftRightMobile/test/services/entity_extractor/url_detector_test.dart
git commit -m "feat(extraction): URL detector — http(s) + bare domains w/ TLD whitelist"
```

---

### Task 2.5: OTP detector

**Files:**
- Create: `DraftRightMobile/lib/services/entity_extractor/otp_detector.dart`
- Modify: `DraftRightMobile/lib/services/entity_extractor.dart`
- Create: `DraftRightMobile/test/services/entity_extractor/otp_detector_test.dart`

- [ ] **Step 1: Failing test**

```dart
// DraftRightMobile/test/services/entity_extractor/otp_detector_test.dart
import 'package:flutter_test/flutter_test.dart';
import 'package:draftright_mobile/services/entity_extractor/otp_detector.dart';
import 'package:draftright_mobile/models/entity.dart';

void main() {
  final det = OtpDetector();
  test('detects when trigger word "OTP" present', () {
    final r = det.detect('Your OTP is 482917 for login');
    expect(r.single.kind, EntityKind.otp);
    expect(r.single.value, '482917');
  });

  test('detects with Vietnamese trigger "mã"', () {
    final r = det.detect('Mã xác minh: 4829');
    expect(r.single.value, '4829');
  });

  test('detects with "mật khẩu" (password)', () {
    final r = det.detect('Mật khẩu wifi 88889999');
    expect(r.single.value, '88889999');
  });

  test('rejects bare 4-digit year (no trigger)', () {
    expect(det.detect('Năm 2024 là năm tốt'), isEmpty);
  });

  test('rejects phone-shaped numbers (10 digits)', () {
    expect(det.detect('Call 0987654321'), isEmpty);
  });
}
```

- [ ] **Step 2: Run, expect FAIL**

- [ ] **Step 3: Implement**

```dart
// DraftRightMobile/lib/services/entity_extractor/otp_detector.dart
import '../../models/entity.dart';
import 'detector.dart';

class OtpDetector implements EntityDetector {
  static final _triggerPattern = RegExp(
    r'(otp|m[ãa]\s*(x[áa]c\s*minh)?|verification|code|m[ậa]t\s*kh[ẩa]u|password)',
    caseSensitive: false,
  );
  static final _digitPattern = RegExp(r'\b\d{4,8}\b');

  @override
  List<Entity> detect(String text) {
    final out = <Entity>[];
    for (final m in _digitPattern.allMatches(text)) {
      // Within 20 chars before the digits, look for a trigger.
      final windowStart = (m.start - 20).clamp(0, text.length);
      final window = text.substring(windowStart, m.start);
      if (!_triggerPattern.hasMatch(window)) continue;
      out.add(Entity(
        kind: EntityKind.otp,
        value: m.group(0)!,
        display: m.group(0)!,
        start: m.start,
        end: m.end,
        source: 'regex',
        confidence: 0.9,
      ));
    }
    return out;
  }
}
```

Register:

```dart
import 'entity_extractor/otp_detector.dart';
// ...
  static final List<EntityDetector> _detectors = <EntityDetector>[
    PhoneDetector(),
    EmailDetector(),
    UrlDetector(),
    OtpDetector(),
  ];
```

- [ ] **Step 4: Run, expect PASS**

- [ ] **Step 5: Commit**

```bash
git add DraftRightMobile/lib/services/entity_extractor/otp_detector.dart \
        DraftRightMobile/lib/services/entity_extractor.dart \
        DraftRightMobile/test/services/entity_extractor/otp_detector_test.dart
git commit -m "feat(extraction): OTP detector — requires trigger word within 20 chars"
```

---

### Task 2.6: Credit card detector (Luhn-checked)

**Files:**
- Create: `DraftRightMobile/lib/services/entity_extractor/credit_card_detector.dart`
- Modify: `DraftRightMobile/lib/services/entity_extractor.dart`
- Create: `DraftRightMobile/test/services/entity_extractor/credit_card_detector_test.dart`

- [ ] **Step 1: Failing test**

```dart
// DraftRightMobile/test/services/entity_extractor/credit_card_detector_test.dart
import 'package:flutter_test/flutter_test.dart';
import 'package:draftright_mobile/services/entity_extractor/credit_card_detector.dart';
import 'package:draftright_mobile/models/entity.dart';

void main() {
  final det = CreditCardDetector();
  test('valid Visa test card passes Luhn', () {
    final r = det.detect('Card 4242 4242 4242 4242 expires 12/27');
    expect(r.single.kind, EntityKind.creditCard);
    expect(r.single.value, '4242424242424242');
    expect(r.single.display, '**** **** **** 4242');
    expect(r.single.meta['masked'], 'true');
  });

  test('invalid Luhn rejected', () {
    expect(det.detect('Bad card 1234 5678 9012 3456'), isEmpty);
  });

  test('dashes accepted', () {
    final r = det.detect('Try 4242-4242-4242-4242 here');
    expect(r.single.value, '4242424242424242');
  });
}
```

- [ ] **Step 2: FAIL**

- [ ] **Step 3: Implement**

```dart
// DraftRightMobile/lib/services/entity_extractor/credit_card_detector.dart
import '../../models/entity.dart';
import 'detector.dart';

class CreditCardDetector implements EntityDetector {
  static final _pattern = RegExp(r'\b(?:\d[\s\-]?){12,18}\d\b');

  @override
  List<Entity> detect(String text) {
    final out = <Entity>[];
    for (final m in _pattern.allMatches(text)) {
      final raw = m.group(0)!;
      final digits = raw.replaceAll(RegExp(r'[\s\-]'), '');
      if (digits.length < 13 || digits.length > 19) continue;
      if (!_luhn(digits)) continue;
      final last4 = digits.substring(digits.length - 4);
      out.add(Entity(
        kind: EntityKind.creditCard,
        value: digits,
        display: '**** **** **** $last4',
        start: m.start,
        end: m.end,
        source: 'regex',
        confidence: 0.99,
        meta: const {'masked': 'true'},
      ));
    }
    return out;
  }

  static bool _luhn(String digits) {
    var sum = 0;
    var doubleIt = false;
    for (var i = digits.length - 1; i >= 0; i--) {
      var d = int.parse(digits[i]);
      if (doubleIt) {
        d *= 2;
        if (d > 9) d -= 9;
      }
      sum += d;
      doubleIt = !doubleIt;
    }
    return sum % 10 == 0;
  }
}
```

Register:

```dart
import 'entity_extractor/credit_card_detector.dart';
// ...
  static final List<EntityDetector> _detectors = <EntityDetector>[
    PhoneDetector(),
    EmailDetector(),
    UrlDetector(),
    OtpDetector(),
    CreditCardDetector(),
  ];
```

- [ ] **Step 4: Run, expect PASS**

- [ ] **Step 5: Commit**

```bash
git add DraftRightMobile/lib/services/entity_extractor/credit_card_detector.dart \
        DraftRightMobile/lib/services/entity_extractor.dart \
        DraftRightMobile/test/services/entity_extractor/credit_card_detector_test.dart
git commit -m "feat(extraction): credit card detector with Luhn check + masking"
```

---

### Task 2.7: Bank account detector (VN banks)

**Files:**
- Create: `DraftRightMobile/lib/services/entity_extractor/bank_catalog.dart`
- Create: `DraftRightMobile/lib/services/entity_extractor/bank_account_detector.dart`
- Modify: `DraftRightMobile/lib/services/entity_extractor.dart`
- Create: `DraftRightMobile/test/services/entity_extractor/bank_account_detector_test.dart`

- [ ] **Step 1: Failing test**

```dart
// DraftRightMobile/test/services/entity_extractor/bank_account_detector_test.dart
import 'package:flutter_test/flutter_test.dart';
import 'package:draftright_mobile/services/entity_extractor/bank_account_detector.dart';
import 'package:draftright_mobile/models/entity.dart';

void main() {
  final det = BankAccountDetector();

  test('detects Vietcombank account', () {
    final r = det.detect('Chuyển khoản Vietcombank 0123456789 giúp anh');
    expect(r.single.kind, EntityKind.bankAccount);
    expect(r.single.value, '0123456789');
    expect(r.single.meta['bank'], 'Vietcombank');
    expect(r.single.display, 'Vietcombank · 0123456789');
  });

  test('detects MB Bank lowercase', () {
    final r = det.detect('mb 9876543210');
    expect(r.single.meta['bank'], 'MB');
  });

  test('rejects standalone numbers without bank context', () {
    expect(det.detect('Số là 0123456789'), isEmpty);
  });

  test('rejects bank name without nearby account', () {
    expect(
      det.detect('Tôi xài Vietcombank rất nhiều, nhưng không nhớ số tài khoản'),
      isEmpty,
    );
  });
}
```

- [ ] **Step 2: FAIL**

- [ ] **Step 3: Implement**

```dart
// DraftRightMobile/lib/services/entity_extractor/bank_catalog.dart
class BankCatalog {
  /// Map of normalized bank-name alias -> display name.
  /// Ordered longest first to avoid prefix matches (Vietcombank beats Viet).
  static final Map<String, String> aliases = {
    'vietcombank': 'Vietcombank',
    'techcombank': 'Techcombank',
    'vietinbank': 'VietinBank',
    'sacombank': 'Sacombank',
    'agribank': 'Agribank',
    'vpbank': 'VPBank',
    'bidv': 'BIDV',
    'tpbank': 'TPBank',
    'mbbank': 'MB',
    'mb bank': 'MB',
    'acb': 'ACB',
    'ocb': 'OCB',
    'mb': 'MB',
  };
}
```

```dart
// DraftRightMobile/lib/services/entity_extractor/bank_account_detector.dart
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
```

Register:

```dart
import 'entity_extractor/bank_account_detector.dart';
// ...
  static final List<EntityDetector> _detectors = <EntityDetector>[
    PhoneDetector(),
    EmailDetector(),
    UrlDetector(),
    OtpDetector(),
    CreditCardDetector(),
    BankAccountDetector(),
  ];
```

- [ ] **Step 4: Run, expect PASS** (all 4 tests)

- [ ] **Step 5: Commit**

```bash
git add DraftRightMobile/lib/services/entity_extractor/bank_account_detector.dart \
        DraftRightMobile/lib/services/entity_extractor/bank_catalog.dart \
        DraftRightMobile/lib/services/entity_extractor.dart \
        DraftRightMobile/test/services/entity_extractor/bank_account_detector_test.dart
git commit -m "feat(extraction): bank account detector with VN bank catalog"
```

---

### Task 2.8: Integration test — mixed message

**Files:**
- Modify: `DraftRightMobile/test/services/entity_extractor_test.dart` (append)

- [ ] **Step 1: Add integration test**

Append to `test/services/entity_extractor_test.dart`:

```dart
  group('EntityExtractor — integration', () {
    test('mixed message: phone + email + url + bank + OTP', () {
      const text =
          'Vietcombank 0123456789 — gọi 0912 345 678, email tan@x.com, '
          'web shop.com, OTP 482917';
      final out = EntityExtractor.extract(text);
      final kinds = out.map((e) => e.kind).toSet();
      expect(kinds, containsAll([
        EntityKind.bankAccount,
        EntityKind.phone,
        EntityKind.email,
        EntityKind.url,
        EntityKind.otp,
      ]));
    });

    test('sorted ascending by start offset', () {
      const text = 'tan@x.com then 0912345678';
      final out = EntityExtractor.extract(text);
      expect(out.first.kind, EntityKind.email);
      expect(out.last.kind, EntityKind.phone);
    });

    test('every entity offset round-trips', () {
      const text =
          'Call 0912345678 or +84912345678. Email tan@x.com. Web shop.com.';
      for (final e in EntityExtractor.extract(text)) {
        expect(text.substring(e.start, e.end).contains(e.display), isTrue,
            reason: 'display "${e.display}" not found at ${e.start}..${e.end}');
      }
    });
  });
```

- [ ] **Step 2: Run, expect PASS**

```bash
cd DraftRightMobile && flutter test test/services/entity_extractor_test.dart
```

- [ ] **Step 3: Commit**

```bash
git add DraftRightMobile/test/services/entity_extractor_test.dart
git commit -m "test(extraction): integration test for mixed-entity input"
```

---

## Phase 3 — Dart API client

### Task 3.1: `ExtractionApi` client + custom exceptions

**Files:**
- Create: `DraftRightMobile/lib/services/extraction_api.dart`
- Create: `DraftRightMobile/test/services/extraction_api_test.dart`

- [ ] **Step 1: Failing test**

```dart
// DraftRightMobile/test/services/extraction_api_test.dart
import 'package:flutter_test/flutter_test.dart';
import 'package:draftright_mobile/services/extraction_api.dart';
import 'package:draftright_mobile/models/entity.dart';
import 'package:http/http.dart' as http;

class _FakeClient implements http.Client {
  _FakeClient(this.responder);
  final http.Response Function(http.Request) responder;

  @override
  Future<http.Response> post(Uri url,
      {Map<String, String>? headers,
      Object? body,
      Encoding? encoding}) async {
    final req = http.Request('POST', url)
      ..body = body is String ? body : '';
    return responder(req);
  }

  @override
  void close() {}

  @override
  noSuchMethod(Invocation i) =>
      throw UnsupportedError(i.memberName.toString());
}

void main() {
  test('200 returns parsed entities with source=llm', () async {
    final api = ExtractionApi(
      baseUrl: 'https://api.test',
      tokenProvider: () async => 'jwt',
      httpClient: _FakeClient((_) => http.Response(
            '{"entities":[{"kind":"address","value":"123 Lê Lợi","display":"123 Lê Lợi","start":0,"end":10,"confidence":0.8}],"provider":"openai","tokensUsed":50}',
            200,
          )),
    );
    final out = await api.llmExtract('whatever');
    expect(out.single.kind, EntityKind.address);
    expect(out.single.source, 'llm');
  });

  test('401 throws ExtractionUnavailableException', () async {
    final api = ExtractionApi(
      baseUrl: 'https://api.test',
      tokenProvider: () async => 'jwt',
      httpClient: _FakeClient((_) => http.Response('unauthorized', 401)),
    );
    expect(api.llmExtract('x'),
        throwsA(isA<ExtractionUnavailableException>()));
  });

  test('402 throws ExtractionQuotaException', () async {
    final api = ExtractionApi(
      baseUrl: 'https://api.test',
      tokenProvider: () async => 'jwt',
      httpClient: _FakeClient((_) => http.Response('quota', 402)),
    );
    expect(api.llmExtract('x'), throwsA(isA<ExtractionQuotaException>()));
  });

  test('500 throws ExtractionUnavailableException', () async {
    final api = ExtractionApi(
      baseUrl: 'https://api.test',
      tokenProvider: () async => 'jwt',
      httpClient: _FakeClient((_) => http.Response('boom', 500)),
    );
    expect(api.llmExtract('x'),
        throwsA(isA<ExtractionUnavailableException>()));
  });
}
```

- [ ] **Step 2: FAIL**

```bash
cd DraftRightMobile && flutter test test/services/extraction_api_test.dart
```

- [ ] **Step 3: Implement**

```dart
// DraftRightMobile/lib/services/extraction_api.dart
import 'dart:async';
import 'dart:convert';
import 'package:http/http.dart' as http;
import '../models/entity.dart';

class ExtractionUnavailableException implements Exception {
  ExtractionUnavailableException(this.reason);
  final String reason;
  @override
  String toString() => 'ExtractionUnavailableException: $reason';
}

class ExtractionQuotaException implements Exception {
  ExtractionQuotaException();
  @override
  String toString() => 'ExtractionQuotaException';
}

class ExtractionApi {
  ExtractionApi({
    required this.baseUrl,
    required this.tokenProvider,
    http.Client? httpClient,
    Duration? timeout,
  })  : _http = httpClient ?? http.Client(),
        _timeout = timeout ?? const Duration(seconds: 10);

  final String baseUrl;
  final Future<String?> Function() tokenProvider;
  final http.Client _http;
  final Duration _timeout;

  Future<List<Entity>> llmExtract(String text) async {
    final token = await tokenProvider();
    if (token == null || token.isEmpty) {
      throw ExtractionUnavailableException('missing auth token');
    }
    final url = Uri.parse('${_strip(baseUrl)}/extract');
    final body = jsonEncode({'text': text});
    final http.Response resp;
    try {
      resp = await _http
          .post(url,
              headers: {
                'Content-Type': 'application/json',
                'Authorization': 'Bearer $token',
              },
              body: body)
          .timeout(_timeout);
    } on TimeoutException {
      throw ExtractionUnavailableException('timeout');
    } catch (e) {
      throw ExtractionUnavailableException('network: $e');
    }

    if (resp.statusCode == 402) throw ExtractionQuotaException();
    if (resp.statusCode == 401 || resp.statusCode == 403) {
      throw ExtractionUnavailableException('auth: ${resp.statusCode}');
    }
    if (resp.statusCode < 200 || resp.statusCode >= 300) {
      throw ExtractionUnavailableException('http: ${resp.statusCode}');
    }
    final Map<String, dynamic> json;
    try {
      json = jsonDecode(resp.body) as Map<String, dynamic>;
    } catch (_) {
      throw ExtractionUnavailableException('malformed response');
    }
    final list = (json['entities'] as List?) ?? const [];
    return list
        .map((raw) {
          final m = Map<String, dynamic>.from(raw as Map);
          m['source'] = 'llm';
          return Entity.fromJson(m);
        })
        .toList();
  }

  static String _strip(String s) => s.endsWith('/') ? s.substring(0, s.length - 1) : s;
}
```

- [ ] **Step 4: Run, expect PASS** (4 tests)

- [ ] **Step 5: Commit**

```bash
git add DraftRightMobile/lib/services/extraction_api.dart \
        DraftRightMobile/test/services/extraction_api_test.dart
git commit -m "feat(extraction): ExtractionApi client with typed exceptions"
```

---

## Phase 4 — Flutter UI

### Task 4.1: `EntitySheetScreen` widget

**Files:**
- Create: `DraftRightMobile/lib/screens/entity_sheet_screen.dart`
- Create: `DraftRightMobile/test/screens/entity_sheet_screen_test.dart`

- [ ] **Step 1: Failing widget test**

```dart
// DraftRightMobile/test/screens/entity_sheet_screen_test.dart
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:draftright_mobile/models/entity.dart';
import 'package:draftright_mobile/screens/entity_sheet_screen.dart';

void main() {
  testWidgets('renders one group per kind, copy button works', (tester) async {
    final initial = [
      Entity(
        kind: EntityKind.phone,
        value: '+84912345678',
        display: '0912 345 678',
        start: 0,
        end: 11,
        source: 'regex',
        confidence: 1.0,
      ),
      Entity(
        kind: EntityKind.email,
        value: 'tan@x.com',
        display: 'tan@x.com',
        start: 15,
        end: 24,
        source: 'regex',
        confidence: 1.0,
      ),
    ];

    String? copied;
    TestDefaultBinaryMessengerBinding.instance.defaultBinaryMessenger
        .setMockMethodCallHandler(SystemChannels.platform, (call) async {
      if (call.method == 'Clipboard.setData') {
        copied = (call.arguments as Map)['text'] as String;
      }
      return null;
    });

    await tester.pumpWidget(MaterialApp(
      home: EntitySheetScreen(
        text: 'hello',
        initial: initial,
        smartScan: null,
      ),
    ));

    expect(find.text('0912 345 678'), findsOneWidget);
    expect(find.text('tan@x.com'), findsOneWidget);
    expect(find.text('Phone'), findsOneWidget);
    expect(find.text('Email'), findsOneWidget);

    await tester.tap(find.byKey(const ValueKey('copy-phone-+84912345678')));
    await tester.pump();
    expect(copied, '+84912345678');
  });

  testWidgets('credit card row shows masked display by default', (tester) async {
    await tester.pumpWidget(MaterialApp(
      home: EntitySheetScreen(
        text: '4242 4242 4242 4242',
        initial: [
          Entity(
            kind: EntityKind.creditCard,
            value: '4242424242424242',
            display: '**** **** **** 4242',
            start: 0,
            end: 19,
            source: 'regex',
            confidence: 1.0,
            meta: const {'masked': 'true'},
          ),
        ],
        smartScan: null,
      ),
    ));
    expect(find.text('**** **** **** 4242'), findsOneWidget);
  });
}
```

- [ ] **Step 2: FAIL**

- [ ] **Step 3: Implement widget**

```dart
// DraftRightMobile/lib/screens/entity_sheet_screen.dart
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import '../models/entity.dart';
import '../services/extraction_api.dart';

/// Optional smart-scan callback. If null, the Smart-scan button is hidden.
typedef SmartScanFn = Future<List<Entity>> Function(String text);

class EntitySheetScreen extends StatefulWidget {
  const EntitySheetScreen({
    super.key,
    required this.text,
    required this.initial,
    required this.smartScan,
  });

  final String text;
  final List<Entity> initial;
  final SmartScanFn? smartScan;

  @override
  State<EntitySheetScreen> createState() => _EntitySheetScreenState();
}

class _EntitySheetScreenState extends State<EntitySheetScreen> {
  late List<Entity> entities;
  bool smartScanLoading = false;
  bool smartScanDone = false;

  @override
  void initState() {
    super.initState();
    entities = List.of(widget.initial);
    assert(entities.isNotEmpty,
        'EntitySheetScreen must not be mounted with empty entities');
  }

  Map<EntityKind, List<Entity>> get _grouped {
    final map = <EntityKind, List<Entity>>{};
    for (final e in entities) {
      map.putIfAbsent(e.kind, () => []).add(e);
    }
    return map;
  }

  Future<void> _onSmartScan() async {
    if (widget.smartScan == null || smartScanLoading) return;
    setState(() => smartScanLoading = true);
    try {
      final llm = await widget.smartScan!(widget.text);
      final merged = _merge(entities, llm);
      final added = merged.length - entities.length;
      setState(() {
        entities = merged;
        smartScanDone = true;
        smartScanLoading = false;
      });
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(content: Text(added > 0
              ? 'Found $added more'
              : 'No additional entities found')),
        );
      }
    } on ExtractionQuotaException {
      setState(() => smartScanLoading = false);
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          const SnackBar(content: Text('Smart scan limit reached')),
        );
      }
    } catch (_) {
      setState(() => smartScanLoading = false);
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          const SnackBar(content: Text('Smart scan unavailable — using basic results')),
        );
      }
    }
  }

  List<Entity> _merge(List<Entity> a, List<Entity> b) {
    final seen = {for (final e in a) e.dedupeKey};
    return [...a, ...b.where((e) => seen.add(e.dedupeKey))];
  }

  Future<void> _copy(Entity e) async {
    await Clipboard.setData(ClipboardData(text: e.value));
    if (!mounted) return;
    ScaffoldMessenger.of(context).showSnackBar(
      SnackBar(content: Text('${_kindLabel(e.kind)} copied')),
    );
  }

  String _kindLabel(EntityKind k) => switch (k) {
        EntityKind.phone => 'Phone',
        EntityKind.email => 'Email',
        EntityKind.url => 'URL',
        EntityKind.otp => 'OTP',
        EntityKind.creditCard => 'Card',
        EntityKind.address => 'Address',
        EntityKind.personName => 'Person',
        EntityKind.dateTime => 'Date/time',
        EntityKind.bankAccount => 'Bank account',
      };

  IconData _kindIcon(EntityKind k) => switch (k) {
        EntityKind.phone => Icons.phone,
        EntityKind.email => Icons.email,
        EntityKind.url => Icons.link,
        EntityKind.otp => Icons.password,
        EntityKind.creditCard => Icons.credit_card,
        EntityKind.address => Icons.home,
        EntityKind.personName => Icons.person,
        EntityKind.dateTime => Icons.calendar_today,
        EntityKind.bankAccount => Icons.account_balance,
      };

  @override
  Widget build(BuildContext context) {
    final groups = _grouped;
    final orderedKinds = groups.keys.toList()
      ..sort((a, b) => a.index.compareTo(b.index));

    return Scaffold(
      appBar: AppBar(title: const Text('Extracted info')),
      body: ListView(
        padding: const EdgeInsets.all(12),
        children: [
          ...orderedKinds.map((k) => Card(
                margin: const EdgeInsets.only(bottom: 10),
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Padding(
                      padding: const EdgeInsets.fromLTRB(14, 12, 14, 4),
                      child: Row(
                        children: [
                          Icon(_kindIcon(k), size: 14, color: Colors.grey),
                          const SizedBox(width: 6),
                          Text(_kindLabel(k),
                              style: const TextStyle(
                                  fontSize: 11,
                                  fontWeight: FontWeight.w600,
                                  color: Colors.grey)),
                        ],
                      ),
                    ),
                    ...groups[k]!.map((e) => ListTile(
                          title: Text(e.display),
                          subtitle: e.source == 'llm'
                              ? const Text('AI',
                                  style: TextStyle(
                                      fontSize: 10, color: Colors.purple))
                              : null,
                          trailing: IconButton(
                            key: ValueKey('copy-${k.wireName}-${e.value}'),
                            icon: const Icon(Icons.copy),
                            tooltip: 'Copy',
                            onPressed: () => _copy(e),
                          ),
                        )),
                  ],
                ),
              )),
          if (widget.smartScan != null && !smartScanDone)
            FilledButton.icon(
              onPressed: smartScanLoading ? null : _onSmartScan,
              icon: smartScanLoading
                  ? const SizedBox(
                      width: 14,
                      height: 14,
                      child: CircularProgressIndicator(strokeWidth: 2),
                    )
                  : const Icon(Icons.auto_awesome),
              label: const Text('Smart scan for addresses, names…'),
            ),
        ],
      ),
      bottomNavigationBar: SafeArea(
        child: Padding(
          padding: const EdgeInsets.all(12),
          child: OutlinedButton(
            onPressed: () => Navigator.of(context).maybePop(),
            child: const Text('Done'),
          ),
        ),
      ),
    );
  }
}
```

- [ ] **Step 4: Run, expect PASS**

```bash
cd DraftRightMobile && flutter test test/screens/entity_sheet_screen_test.dart
```

- [ ] **Step 5: Commit**

```bash
git add DraftRightMobile/lib/screens/entity_sheet_screen.dart \
        DraftRightMobile/test/screens/entity_sheet_screen_test.dart
git commit -m "feat(extraction): EntitySheetScreen — grouped list + copy + smart scan"
```

---

### Task 4.2: Wire `ShareRewriteScreen` to branch into entity sheet

**Files:**
- Modify: `DraftRightMobile/lib/screens/share_rewrite_screen.dart`

- [ ] **Step 1: Read the current file to locate the share-received branch point**

```bash
sed -n '1,80p' /opt/openAi/DraftRight/DraftRightMobile/lib/screens/share_rewrite_screen.dart
```

- [ ] **Step 2: Add branching at the screen's `initState` (or wherever `sharedText` is first available)**

Find the place where the screen has `final String sharedText;` available and `initState()` runs. At the top of `initState` (after `super.initState()`), insert:

```dart
import '../models/entity.dart';                                 // add import
import '../services/entity_extractor.dart';                     // add import
import '../services/extraction_api.dart';                       // add import
import 'entity_sheet_screen.dart';                              // add import

// ...

@override
void initState() {
  super.initState();
  // After existing init, branch into entity sheet if entities present.
  WidgetsBinding.instance.addPostFrameCallback((_) {
    final initial = EntityExtractor.extract(widget.sharedText);
    if (initial.isEmpty) return;  // fall through to existing tone picker
    final api = ExtractionApi(
      baseUrl: context.read<BackendConfig>().baseUrl,        // existing provider; see note
      tokenProvider: () async => context.read<AuthService>().jwt,
    );
    Navigator.of(context).pushReplacement(
      MaterialPageRoute(
        builder: (_) => EntitySheetScreen(
          text: widget.sharedText,
          initial: initial,
          smartScan: (txt) => api.llmExtract(txt),
        ),
      ),
    );
  });
}
```

**Note on imports:** the actual names of `BackendConfig` / `AuthService` providers in this project may differ. Read `lib/main.dart` or `lib/services/` to find the canonical provider for (a) the backend base URL and (b) the current JWT. Substitute the correct accessor.

If no Provider-based DI exists, instantiate `ExtractionApi` with values pulled from `SettingsService`:

```dart
final settings = SettingsService();
await settings.load();
final api = ExtractionApi(
  baseUrl: settings.backendUrl,
  tokenProvider: () async => settings.jwt,  // or however JWT is exposed
);
```

- [ ] **Step 3: Manual smoke test**

```bash
cd DraftRightMobile && flutter run -d <device>
# Share a test message from another app: "Call 0912345678 or email tan@x.com"
# Verify: Entity sheet appears with 2 rows.
# Share prose "Hello there": tone picker appears (no regression).
```

- [ ] **Step 4: Commit**

```bash
git add DraftRightMobile/lib/screens/share_rewrite_screen.dart
git commit -m "feat(extraction): branch ShareRewriteScreen into EntitySheet when entities present"
```

---

## Phase 5 — Backend `POST /extract`

### Task 5.1: DTOs

**Files:**
- Create: `backend/src/extraction/dto/extract.dto.ts`

- [ ] **Step 1: Implement DTOs (no test needed — pure type definitions, exercised by service/controller tests)**

```typescript
// backend/src/extraction/dto/extract.dto.ts
import {
  IsArray,
  IsEnum,
  IsNumber,
  IsObject,
  IsOptional,
  IsString,
  MaxLength,
  ArrayMaxSize,
} from 'class-validator';

export enum EntityKind {
  Phone = 'phone',
  Email = 'email',
  Url = 'url',
  Otp = 'otp',
  CreditCard = 'creditCard',
  Address = 'address',
  PersonName = 'personName',
  DateTime = 'dateTime',
  BankAccount = 'bankAccount',
}

export class ExtractRequestDto {
  @IsString()
  @MaxLength(8000)
  text!: string;

  @IsOptional()
  @IsArray()
  @ArrayMaxSize(20)
  @IsEnum(EntityKind, { each: true })
  kinds?: EntityKind[];
}

export interface ExtractedEntityDto {
  kind: EntityKind;
  value: string;
  display: string;
  start: number;
  end: number;
  confidence: number;
  meta?: Record<string, string>;
}

export interface ExtractResponseDto {
  entities: ExtractedEntityDto[];
  provider: string;
  tokensUsed: number;
}
```

- [ ] **Step 2: Commit**

```bash
git add backend/src/extraction/dto/extract.dto.ts
git commit -m "feat(backend/extraction): DTOs for POST /extract"
```

---

### Task 5.2: `ExtractionService` — prompt + offset recovery + dedupe

**Files:**
- Create: `backend/src/extraction/extraction.service.ts`
- Create: `backend/src/extraction/extraction.service.spec.ts`

- [ ] **Step 1: Failing test**

```typescript
// backend/src/extraction/extraction.service.spec.ts
import { Test } from '@nestjs/testing';
import { ExtractionService } from './extraction.service';
import { AiProvidersService } from '../ai-providers/ai-providers.service';
import { EntityKind } from './dto/extract.dto';

describe('ExtractionService', () => {
  let service: ExtractionService;
  let aiProviders: { findDefault: jest.Mock; callProvider: jest.Mock };

  beforeEach(async () => {
    aiProviders = {
      findDefault: jest.fn().mockResolvedValue({
        id: 'p1',
        type: 'openai',
        name: 'openai',
        model: 'gpt-4o-mini',
        is_active: true,
        endpoint_url: 'http://x',
        api_key: 'x',
        temperature: 0.2,
      }),
      callProvider: jest.fn(),
    };
    const mod = await Test.createTestingModule({
      providers: [
        ExtractionService,
        { provide: AiProvidersService, useValue: aiProviders },
      ],
    }).compile();
    service = mod.get(ExtractionService);
  });

  it('returns empty when LLM responds with non-JSON', async () => {
    aiProviders.callProvider.mockResolvedValue({ text: 'not json', responseTimeMs: 5 });
    const out = await service.extract('hello world');
    expect(out.entities).toEqual([]);
  });

  it('drops entities whose value is not in original text (hallucination guard)', async () => {
    aiProviders.callProvider.mockResolvedValue({
      text: JSON.stringify([
        { kind: 'address', value: '123 Lê Lợi', display: '123 Lê Lợi', confidence: 0.8 },
        { kind: 'address', value: 'FAKE STREET', display: 'FAKE STREET', confidence: 0.8 },
      ]),
      responseTimeMs: 10,
    });
    const out = await service.extract('Địa chỉ 123 Lê Lợi');
    expect(out.entities).toHaveLength(1);
    expect(out.entities[0].value).toBe('123 Lê Lợi');
  });

  it('recomputes offsets via indexOf', async () => {
    aiProviders.callProvider.mockResolvedValue({
      text: JSON.stringify([
        { kind: 'address', value: '123 Lê Lợi', display: '123 Lê Lợi', confidence: 0.8, start: 999, end: 1010 },
      ]),
      responseTimeMs: 10,
    });
    const out = await service.extract('Địa chỉ 123 Lê Lợi');
    expect(out.entities[0].start).toBe(8);
    expect(out.entities[0].end).toBe(8 + '123 Lê Lợi'.length);
  });

  it('drops disallowed kinds (regex-handled set)', async () => {
    aiProviders.callProvider.mockResolvedValue({
      text: JSON.stringify([
        { kind: 'phone', value: '0912345678', display: '0912345678', confidence: 0.9 },
        { kind: 'address', value: '123 Lê Lợi', display: '123 Lê Lợi', confidence: 0.8 },
      ]),
      responseTimeMs: 10,
    });
    const out = await service.extract('phone 0912345678 at 123 Lê Lợi');
    const kinds = out.entities.map((e) => e.kind);
    expect(kinds).not.toContain('phone');
    expect(kinds).toContain('address');
  });
});
```

- [ ] **Step 2: FAIL**

```bash
cd backend && npx jest src/extraction/extraction.service.spec.ts
```

- [ ] **Step 3: Implement**

```typescript
// backend/src/extraction/extraction.service.ts
import { Injectable, Logger } from '@nestjs/common';
import { AiProvidersService } from '../ai-providers/ai-providers.service';
import {
  EntityKind,
  ExtractResponseDto,
  ExtractedEntityDto,
} from './dto/extract.dto';

const REGEX_HANDLED = new Set<EntityKind>([
  EntityKind.Phone,
  EntityKind.Email,
  EntityKind.Url,
  EntityKind.Otp,
  EntityKind.CreditCard,
]);

@Injectable()
export class ExtractionService {
  private readonly logger = new Logger(ExtractionService.name);

  constructor(private readonly aiProviders: AiProvidersService) {}

  async extract(
    text: string,
    kinds?: EntityKind[],
  ): Promise<ExtractResponseDto> {
    const provider = await this.aiProviders.findDefault();
    const system = this.buildSystemPrompt(kinds);
    const user = text;
    const { text: rawOutput, responseTimeMs } =
      await this.aiProviders.callProvider(provider, system, user);

    let parsed: any;
    try {
      const cleaned = this.stripCodeFences(rawOutput);
      parsed = JSON.parse(cleaned);
    } catch (e) {
      this.logger.warn(`extraction_llm_unparseable: ${rawOutput.slice(0, 200)}`);
      return { entities: [], provider: provider.name, tokensUsed: 0 };
    }
    if (!Array.isArray(parsed)) {
      return { entities: [], provider: provider.name, tokensUsed: 0 };
    }

    const out: ExtractedEntityDto[] = [];
    for (const raw of parsed) {
      const v = this.validateEntity(raw, text);
      if (v) out.push(v);
    }
    return {
      entities: this.dedupe(out),
      provider: provider.name,
      tokensUsed: this.estimateTokens(text + rawOutput),
    };
  }

  private buildSystemPrompt(kinds?: EntityKind[]): string {
    const allowed = (kinds ?? [
      EntityKind.Address,
      EntityKind.PersonName,
      EntityKind.DateTime,
      EntityKind.BankAccount,
    ]).filter((k) => !REGEX_HANDLED.has(k));
    return [
      'You extract structured entities from short messages.',
      'Return strict JSON array, no commentary. No code fences.',
      'Each item: {kind, value, display, confidence, meta?}.',
      `Kinds you MAY emit: ${allowed.join('|')}.`,
      `Kinds you MUST NOT emit (handled by client regex): ${[...REGEX_HANDLED].join('|')}.`,
      'value MUST be a literal substring of the input. confidence is 0..1.',
      'Example input: "Địa chỉ 123 Lê Lợi, Q1. Vietcombank 0123456789"',
      'Example output: [{"kind":"address","value":"123 Lê Lợi, Q1","display":"123 Lê Lợi, Q1","confidence":0.9},{"kind":"bankAccount","value":"0123456789","display":"Vietcombank · 0123456789","confidence":0.95,"meta":{"bank":"Vietcombank"}}]',
    ].join('\n');
  }

  private stripCodeFences(s: string): string {
    const trimmed = s.trim();
    if (trimmed.startsWith('```')) {
      const firstNl = trimmed.indexOf('\n');
      const body = firstNl < 0 ? '' : trimmed.slice(firstNl + 1);
      const endIdx = body.lastIndexOf('```');
      return endIdx >= 0 ? body.slice(0, endIdx).trim() : body.trim();
    }
    return trimmed;
  }

  private validateEntity(raw: any, text: string): ExtractedEntityDto | null {
    if (typeof raw !== 'object' || raw === null) return null;
    const kind = raw.kind;
    if (!Object.values(EntityKind).includes(kind)) return null;
    if (REGEX_HANDLED.has(kind)) return null;          // defense in depth
    const value = typeof raw.value === 'string' ? raw.value : null;
    if (!value) return null;
    const start = text.indexOf(value);
    if (start < 0) {
      this.logger.warn(`extraction_hallucination: ${kind}=${value}`);
      return null;
    }
    const display =
      typeof raw.display === 'string' && raw.display.trim() ? raw.display : value;
    const confidenceRaw = typeof raw.confidence === 'number' ? raw.confidence : 0.5;
    const confidence = Math.max(0, Math.min(1, confidenceRaw));
    const meta =
      raw.meta && typeof raw.meta === 'object'
        ? Object.fromEntries(
            Object.entries(raw.meta).map(([k, v]) => [String(k), String(v)]),
          )
        : undefined;
    return {
      kind,
      value,
      display,
      start,
      end: start + value.length,
      confidence,
      meta,
    };
  }

  private dedupe(items: ExtractedEntityDto[]): ExtractedEntityDto[] {
    const byKey = new Map<string, ExtractedEntityDto>();
    for (const e of items) {
      const key = `${e.kind}:${e.value.toLowerCase()}`;
      const cur = byKey.get(key);
      if (!cur || e.confidence > cur.confidence) byKey.set(key, e);
    }
    return [...byKey.values()].sort((a, b) => a.start - b.start);
  }

  private estimateTokens(s: string): number {
    return Math.ceil(s.length / 4);   // rough heuristic; replace later
  }
}
```

- [ ] **Step 4: Run, expect PASS** (4 tests)

- [ ] **Step 5: Commit**

```bash
git add backend/src/extraction/extraction.service.ts backend/src/extraction/extraction.service.spec.ts
git commit -m "feat(backend/extraction): ExtractionService — prompt + validation + dedupe"
```

---

### Task 5.3: `ExtractionController` + e2e tests

**Files:**
- Create: `backend/src/extraction/extraction.controller.ts`
- Create: `backend/src/extraction/extraction.controller.spec.ts`

- [ ] **Step 1: Failing test**

```typescript
// backend/src/extraction/extraction.controller.spec.ts
import { Test } from '@nestjs/testing';
import { INestApplication, ValidationPipe } from '@nestjs/common';
import * as request from 'supertest';
import { ExtractionController } from './extraction.controller';
import { ExtractionService } from './extraction.service';
import { JwtAuthGuard } from '../auth/jwt-auth.guard';

describe('ExtractionController (e2e)', () => {
  let app: INestApplication;
  let service: { extract: jest.Mock };

  beforeAll(async () => {
    service = {
      extract: jest.fn().mockResolvedValue({
        entities: [
          { kind: 'address', value: '123 Lê Lợi', display: '123 Lê Lợi', start: 0, end: 10, confidence: 0.8 },
        ],
        provider: 'openai',
        tokensUsed: 50,
      }),
    };
    const mod = await Test.createTestingModule({
      controllers: [ExtractionController],
      providers: [{ provide: ExtractionService, useValue: service }],
    })
      .overrideGuard(JwtAuthGuard)
      .useValue({ canActivate: () => true })
      .compile();
    app = mod.createNestApplication();
    app.useGlobalPipes(new ValidationPipe({ whitelist: true, transform: true }));
    await app.init();
  });

  afterAll(async () => app.close());

  it('200 with valid body', async () => {
    const resp = await request(app.getHttpServer())
      .post('/extract')
      .send({ text: '123 Lê Lợi' });
    expect(resp.status).toBe(201);  // Nest default for POST; will be set to 200 via @HttpCode
    expect(resp.body.entities).toHaveLength(1);
  });

  it('400 when text is missing', async () => {
    const resp = await request(app.getHttpServer())
      .post('/extract')
      .send({});
    expect(resp.status).toBe(400);
  });

  it('400 when text exceeds 8000 chars', async () => {
    const resp = await request(app.getHttpServer())
      .post('/extract')
      .send({ text: 'a'.repeat(8001) });
    expect(resp.status).toBe(400);
  });
});
```

Note: project convention per memory note `feedback_nest_post_status.md` — non-creating POSTs return HTTP 200 via `@HttpCode(200)`. Update the test to expect 200 once the controller uses the decorator (see Step 3).

- [ ] **Step 2: FAIL**

- [ ] **Step 3: Implement controller**

```typescript
// backend/src/extraction/extraction.controller.ts
import {
  Body,
  Controller,
  HttpCode,
  Post,
  Req,
  UseGuards,
} from '@nestjs/common';
import { ApiBearerAuth, ApiTags } from '@nestjs/swagger';
import { Request } from 'express';
import { JwtAuthGuard } from '../auth/jwt-auth.guard';
import { ExtractRequestDto, ExtractResponseDto } from './dto/extract.dto';
import { ExtractionService } from './extraction.service';

@ApiTags('extraction')
@Controller('extract')
export class ExtractionController {
  constructor(private readonly extractionService: ExtractionService) {}

  @UseGuards(JwtAuthGuard)
  @ApiBearerAuth()
  @HttpCode(200)
  @Post()
  async extract(@Req() _req: Request, @Body() dto: ExtractRequestDto): Promise<ExtractResponseDto> {
    return this.extractionService.extract(dto.text, dto.kinds);
  }
}
```

Update the test's expected status:

```diff
-    expect(resp.status).toBe(201);  // Nest default for POST; will be set to 200 via @HttpCode
+    expect(resp.status).toBe(200);
```

- [ ] **Step 4: Run, expect PASS** (3 tests)

```bash
cd backend && npx jest src/extraction/extraction.controller.spec.ts
```

- [ ] **Step 5: Commit**

```bash
git add backend/src/extraction/extraction.controller.ts \
        backend/src/extraction/extraction.controller.spec.ts
git commit -m "feat(backend/extraction): POST /extract controller w/ JWT guard + @HttpCode(200)"
```

---

### Task 5.4: Wire ExtractionModule into AppModule

**Files:**
- Create: `backend/src/extraction/extraction.module.ts`
- Modify: `backend/src/app.module.ts`

- [ ] **Step 1: Create module file**

```typescript
// backend/src/extraction/extraction.module.ts
import { Module } from '@nestjs/common';
import { ExtractionController } from './extraction.controller';
import { ExtractionService } from './extraction.service';
import { AiProvidersModule } from '../ai-providers/ai-providers.module';
import { AuthModule } from '../auth/auth.module';

@Module({
  imports: [AiProvidersModule, AuthModule],
  controllers: [ExtractionController],
  providers: [ExtractionService],
})
export class ExtractionModule {}
```

- [ ] **Step 2: Register in AppModule**

Modify `backend/src/app.module.ts` — add to `imports` array:

```diff
   imports: [
     EmailModule,
     HealthModule,
     ...
     ErrorsModule,
     BugReportsModule,
+    ExtractionModule,
   ],
```

And the import line at top:

```diff
+import { ExtractionModule } from './extraction/extraction.module';
```

- [ ] **Step 3: Boot sanity check**

```bash
cd backend && npm run build && npm run start:dev
# In another terminal:
curl -X POST http://localhost:3000/extract \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer <valid-jwt>" \
  -d '{"text":"Địa chỉ 123 Lê Lợi"}'
```
Expected: HTTP 200, JSON with `entities` array.

If the default AI provider is Ollama and not running, the call returns `entities: []` (Ollama unreachable → AiProviders throws → controller bubbles 500). Switch the default provider in admin → AI Providers, or start Ollama:

```bash
open /Applications/Ollama.app
ollama pull llama3.2
```

- [ ] **Step 4: Commit**

```bash
git add backend/src/extraction/extraction.module.ts backend/src/app.module.ts
git commit -m "feat(backend/extraction): wire ExtractionModule into AppModule"
```

---

## Phase 6 — Integration + QA + Deploy

### Task 6.1: Type-check both packages

- [ ] **Step 1: Backend type-check**

```bash
cd /opt/openAi/DraftRight/backend && npx tsc --noEmit
```
Expected: zero errors.

- [ ] **Step 2: Flutter analyze**

```bash
cd /opt/openAi/DraftRight/DraftRightMobile && flutter analyze
```
Expected: zero errors (warnings about unused imports are tolerated only for pre-existing code).

- [ ] **Step 3: Full test runs**

```bash
cd /opt/openAi/DraftRight/backend && npm test -- src/extraction
cd /opt/openAi/DraftRight/DraftRightMobile && flutter test
```
Expected: all green.

- [ ] **Step 4:** No commit — verification only. Move on if green.

---

### Task 6.2: Manual QA on a phone (Android first, then iOS)

- [ ] **Step 1:** Spin up backend locally **OR** point the app at the testing server (Settings → Backend URL). Confirm `/health` returns 200.

- [ ] **Step 2:** Walk every test case `EXTRACT-001` through `EXTRACT-010` on a real Android device or emulator. Record pass/fail in `docs/test-cases.xlsx`.

- [ ] **Step 3:** Repeat on iOS device/simulator. Pay extra attention to **EXTRACT-008** (share-extension routing) and clipboard interactions.

- [ ] **Step 4:** If any test case fails, fix in a new TDD cycle (add a failing test, fix, commit) — do not skip ahead.

---

### Task 6.3: Merge feature → develop, deploy to testing

- [ ] **Step 1:** Merge with explicit no-fast-forward

```bash
cd /opt/openAi/DraftRight
git checkout develop
git pull
git merge --no-ff feature/entity-extraction-20260514 -m "Merge feature/entity-extraction-20260514: entity extraction for shared messages"
git push origin develop
```

- [ ] **Step 2:** Deploy backend to testing server per existing pipeline (see `deploy/CLAUDE.md`). After deploy, smoke-test:

```bash
curl -X POST https://<testing-backend>/extract \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer <testing-jwt>" \
  -d '{"text":"Vietcombank 0123456789"}'
```
Expected: HTTP 200, response contains the address/bank account entity if Smart-scan path is exercised (Ollama / configured provider must be reachable).

- [ ] **Step 3:** Build a testing-flavor Flutter APK / TestFlight build that targets the testing backend. Distribute to QA.

- [ ] **Step 4:** Apply `status: deployed to testing` label on the tracking GitHub issue + comment what was deployed.

---

### Task 6.4: E2E suite against testing server

- [ ] **Step 1:** Run the existing E2E suite (or the manual test cases from 6.2) against the testing backend URL. Record results.

- [ ] **Step 2:** If pass: continue to 6.5. If fail: branch off `develop`, fix in a new `fix/...` branch, merge back, redeploy testing.

---

### Task 6.5: Merge develop → main, deploy to production

- [ ] **Step 1:** Merge

```bash
git checkout main
git pull
git merge --no-ff develop -m "Merge develop → main: entity extraction"
git push origin main
```

- [ ] **Step 2:** Deploy to production per `deploy/CLAUDE.md`. Migrations: none (no new DB tables). Just rebuild and restart the backend container.

- [ ] **Step 3:** Post-deploy health check

```bash
docker ps                                    # all healthy
docker logs draftright-backend --tail 100    # no boot errors
curl https://api.draftright.info/health      # HTTP 200
```

- [ ] **Step 4:** Smoke-test production with a real JWT (any user account):

```bash
curl -X POST https://api.draftright.info/extract \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer <prod-jwt>" \
  -d '{"text":"Địa chỉ 123 Lê Lợi"}'
```
Expected: HTTP 200, valid JSON shape.

- [ ] **Step 5:** Apply `status: deployed to production` label on the tracking issue + add the verification comment:

```markdown
## ✅ How to Verify

**URL:** Open DraftRight Mobile app (Android or iOS), share any chat message into DraftRight.

**What changed:**
- New entity sheet replaces the tone picker when a shared message contains phone / email / URL / OTP / card / VN bank account.
- Tap copy icon next to each entity to copy the canonical value.
- Tap ✨ Smart scan to use AI to extract addresses, names, dates.
- Falls back to tone picker if no entities are detected (no regression).

**Steps to verify:**
1. Update the app to the latest build.
2. From any chat (WhatsApp, Messenger, SMS, etc.), long-press a message that contains a phone number and an email. Tap **Share** → **DraftRight**.
3. Expect the entity sheet with two rows: 📞 Phone and ✉ Email. Tap copy on the phone → SnackBar "Phone copied".
4. Paste into another app — should match the original phone number.
5. Tap **✨ Smart scan** — expect a spinner, then 1+ AI-tagged rows (Address / Person / DateTime) if the message contains them.
6. To verify the regression case: share a plain-text message with no entities ("Hello, hope you are well") → expect the existing tone picker, not the entity sheet.
```

- [ ] **Step 6:** Leave the issue **open** — only the boss closes after manual verification.

---

## Self-Review Checklist

Run through this once before marking the plan complete:

- [ ] Spec §3 goals — every goal has at least one task that implements it.
- [ ] Spec §4 user flow — all three branches (happy / smart-scan / no-entities) covered by Tasks 4.1, 4.2, 5.3.
- [ ] Spec §6 components — every named module/class has a corresponding task.
- [ ] Spec §7 error handling — all rows are testable via tasks 3.1 (client errors) + 5.2 (server errors). UI snackbars exercised in Task 4.1 widget tests.
- [ ] Spec §8 security — JWT guard wired in 5.3; rate limit reuses existing `JwtAuthGuard` chain via 5.4; log masking is **not** in this plan and is added as a follow-up note below.
- [ ] Spec §9 testing — all unit/widget/e2e sections covered. Manual QA in 6.2.
- [ ] No "TBD", no "implement later", no orphan references.

### Known follow-up (NOT blocking)

- Logging interceptor that masks `text` field on `POST /extract` access logs (spec §8). The default NestJS logger does not log POST bodies, so this is informational unless `Morgan` / a custom interceptor is later added. Track as a separate ticket.

---

## Done criteria

- All tasks above checked off.
- All test cases EXTRACT-001..010 pass on the production build.
- Tracking GitHub issue has `status: deployed to production` label and a "✅ How to Verify" comment.
- Issue remains **open** — closed only by the boss after acceptance per project policy.
