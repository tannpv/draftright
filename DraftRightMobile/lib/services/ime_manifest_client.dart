import 'dart:convert';
import 'package:http/http.dart' as http;
import 'package:draftright_mobile/models/language_module.dart';

/// Fetches the server-driven language catalog (`GET /ime-packs/manifest`).
/// Keeping the catalog on the server means new languages and updated pack
/// hashes ship without an app release.
class ImeManifestClient {
  ImeManifestClient({required this.baseUrl, http.Client? httpClient})
      : _http = httpClient ?? http.Client();

  final String baseUrl;
  final http.Client _http;

  Future<List<LanguageModule>> fetch() async {
    final res = await _http.get(Uri.parse('$baseUrl/ime-packs/manifest'));
    if (res.statusCode != 200) {
      throw Exception('manifest HTTP ${res.statusCode}');
    }
    final body = jsonDecode(res.body);
    final list = (body is Map<String, dynamic> ? body['languages'] : body) as List;
    return list
        .map((e) => LanguageModule.fromJson(e as Map<String, dynamic>))
        .toList();
  }

  /// Only the candidate languages that carry a downloadable pack.
  Future<List<LanguageModule>> fetchDownloadable() async =>
      (await fetch()).where((m) => m.requiresDownload).toList();
}
