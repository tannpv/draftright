/// A keyboard language from the server-driven catalog (the "language
/// container"). Bundled languages ship in the app; candidate languages carry a
/// downloadable [pack] (a RIME dictionary) the user installs on demand.
class LanguageModule {
  final String id;
  final String displayName;
  final String inputMethod; // passthrough | composition | candidate
  final String engine; // none | composition | rime
  final String layout;
  final bool bundled;
  final LanguagePack? pack;

  const LanguageModule({
    required this.id,
    required this.displayName,
    required this.inputMethod,
    required this.engine,
    required this.layout,
    this.bundled = false,
    this.pack,
  });

  /// A candidate language needs a downloaded pack before it can be enabled.
  bool get requiresDownload => pack != null;

  factory LanguageModule.fromJson(Map<String, dynamic> j) => LanguageModule(
        id: j['id'] as String,
        displayName: j['displayName'] as String,
        inputMethod: (j['inputMethod'] as String?) ?? 'passthrough',
        engine: (j['engine'] as String?) ?? 'none',
        layout: (j['layout'] as String?) ?? 'qwerty',
        bundled: (j['bundled'] as bool?) ?? false,
        pack: j['pack'] == null
            ? null
            : LanguagePack.fromJson(j['pack'] as Map<String, dynamic>),
      );
}

/// The downloadable dictionary pack for a candidate language.
class LanguagePack {
  final String url;
  final int version;
  final int sizeBytes;
  final String sha256;

  const LanguagePack({
    required this.url,
    required this.version,
    required this.sizeBytes,
    required this.sha256,
  });

  /// Human-readable size, e.g. "≈18 MB", for the download affordance.
  String get sizeLabel {
    if (sizeBytes <= 0) return '';
    final mb = sizeBytes / (1024 * 1024);
    return mb >= 1 ? '≈${mb.toStringAsFixed(0)} MB' : '≈${(sizeBytes / 1024).toStringAsFixed(0)} KB';
  }

  factory LanguagePack.fromJson(Map<String, dynamic> j) => LanguagePack(
        url: j['url'] as String,
        version: (j['version'] as num?)?.toInt() ?? 1,
        sizeBytes: (j['sizeBytes'] as num?)?.toInt() ?? 0,
        sha256: (j['sha256'] as String?) ?? '',
      );
}
