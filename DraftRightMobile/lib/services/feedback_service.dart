import 'dart:convert';
import 'dart:io' show Platform;

import 'package:http/http.dart' as http;

/// Posts feature requests to the backend `POST /feedback` endpoint
/// (JSON body, no screenshot). The bug-report counterpart is
/// [BugReportService.submitBugReport].
class FeedbackService {
  /// Production endpoint — mirrors the base used by BugReportService.
  static const String _defaultEndpoint = 'https://api.draftright.info/feedback';

  /// Submit a feature request. Returns true on a 2xx response, false otherwise.
  ///
  /// [title] is a short summary (UI enforces max 80 chars).
  /// [targetPlatform] is one of: playground|mobile|windows|mac|linux.
  /// [description] is the full request body.
  /// [authToken] is sent as a Bearer token when non-null/non-empty;
  /// otherwise [userEmail] (if any) goes in the JSON body for anonymous
  /// users — mirroring the behaviour of BugReportService.
  /// [endpointOverride] redirects the POST (integration tests).
  /// [httpClient] is injectable for unit tests; a fresh client is created
  /// and closed automatically when not provided.
  static Future<bool> submitFeatureRequest({
    required String title,
    required String targetPlatform,
    required String description,
    String? userEmail,
    String? authToken,
    String? endpointOverride,
    http.Client? httpClient,
  }) async {
    final client = httpClient ?? http.Client();
    try {
      final source = _detectSource();
      final body = <String, dynamic>{
        'kind': 'feature',
        'title': title.trim(),
        'target_platform': targetPlatform,
        'description': description.trim(),
        'source': source,
      };

      // Include user_email only for anonymous requests. When a JWT is
      // present the backend extracts the user identity from the token.
      if ((authToken == null || authToken.isEmpty) &&
          userEmail != null &&
          userEmail.trim().isNotEmpty) {
        body['user_email'] = userEmail.trim();
      }

      final headers = <String, String>{
        'Content-Type': 'application/json',
      };
      if (authToken != null && authToken.isNotEmpty) {
        headers['Authorization'] = 'Bearer $authToken';
      }

      final resp = await client.post(
        Uri.parse(endpointOverride ?? _defaultEndpoint),
        headers: headers,
        body: jsonEncode(body),
      );
      return resp.statusCode >= 200 && resp.statusCode < 300;
    } catch (_) {
      return false;
    } finally {
      if (httpClient == null) client.close();
    }
  }

  /// Mirrors BugReportService._detectSource() — safe fallback for tests.
  static String _detectSource() {
    try {
      if (Platform.isIOS) return 'ios-app';
      if (Platform.isAndroid) return 'android-app';
    } catch (_) {/* non-mobile (test host) */}
    return 'android-app';
  }
}
