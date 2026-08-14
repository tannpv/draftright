import 'package:flutter/material.dart';
import 'package:provider/provider.dart';

import '../screens/entity_sheet_screen.dart';
import 'auth_service.dart';
import 'entity_extractor.dart';
import 'extraction_api.dart';
import 'settings_service.dart';

/// One source of truth for launching Smart Extract (#143): run the offline
/// EntityExtractor on [text] and open EntitySheetScreen with the LLM smart-scan
/// callback. Used by BOTH the Playground button and the share-intent entry
/// point, so the two never drift.
Future<void> openEntitySheet(BuildContext context, String text) async {
  final trimmed = text.trim();
  if (trimmed.isEmpty) return;
  final settings = context.read<SettingsService>();
  final auth = context.read<AuthService>();
  final api = ExtractionApi(
    baseUrl: settings.backendUrl,
    tokenProvider: () async => auth.accessToken,
  );
  await Navigator.of(context).push(
    MaterialPageRoute(
      builder: (_) => EntitySheetScreen(
        text: trimmed,
        initial: EntityExtractor.extract(trimmed),
        smartScan: (t) => api.llmExtract(t),
      ),
    ),
  );
}
