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
