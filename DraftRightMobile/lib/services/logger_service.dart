import 'dart:io';
import 'package:flutter/foundation.dart' show debugPrint;
import 'package:path_provider/path_provider.dart';

/// Log severity. Lines are tagged `[LEVEL]` so failures are skimmable and
/// greppable (`grep "\[ERROR\]"`) instead of buried in same-level category
/// chatter. [warn] = something degraded but handled; [error] = an operation
/// failed.
// `off` is threshold-only (used as [DRLogger.minLevel] to silence everything);
// it is never passed as a line's level. Declaration order defines severity
// rank via `.index`: info < warn < error < off.
enum LogLevel { info, warn, error, off }

/// Centralized file logger for DraftRight.
class DRLogger {
  static File? _logFile;

  /// Minimum severity to write, set remotely by the admin portal and pushed to
  /// clients via `/health`'s `client_log_level`. The master verbosity control:
  /// [LogLevel.info] logs everything (default), [LogLevel.off] is the absolute
  /// kill-switch (silences even errors, for privacy/compliance).
  static LogLevel minLevel = LogLevel.info;

  static Future<void> init() async {
    final dir = await getApplicationDocumentsDirectory();
    final logDir = Directory('${dir.path}/logs');
    if (!logDir.existsSync()) logDir.createSync(recursive: true);
    _logFile = File('${logDir.path}/draftright.log');
  }

  static void log(String message,
      {String category = 'APP', LogLevel level = LogLevel.info}) {
    // Remote admin threshold (master): drop anything below it; off drops all.
    if (level.index < minLevel.index) return;
    final timestamp = DateTime.now().toIso8601String();
    // Pad the level so the [CATEGORY] column stays aligned across lines.
    final levelTag = level.name.toUpperCase().padRight(5);
    final line = '[$timestamp] [$levelTag] [$category] $message';
    _logFile?.writeAsStringSync('$line\n', mode: FileMode.append);
    assert(() { debugPrint(line); return true; }());
  }

  /// Log a handled-but-degraded condition at [LogLevel.warn].
  static void warn(String message, {String category = 'APP'}) =>
      log(message, category: category, level: LogLevel.warn);

  /// Log a failed operation at [LogLevel.error].
  static void error(String message, {String category = 'APP'}) =>
      log(message, category: category, level: LogLevel.error);

  /// Applies the backend's `client_log_level` ("off" | "errors" | "warnings" |
  /// "info") as the [minLevel] threshold. Unknown/empty → full logging. No-op
  /// when unchanged; only a genuine change is announced.
  static void setMinLevelFromServer(String? value) {
    final LogLevel newLevel;
    switch (value?.trim().toLowerCase()) {
      case 'off':
        newLevel = LogLevel.off;
        break;
      case 'errors':
      case 'error':
        newLevel = LogLevel.error;
        break;
      case 'warnings':
      case 'warning':
      case 'warn':
        newLevel = LogLevel.warn;
        break;
      default:
        newLevel = LogLevel.info;
    }
    if (newLevel == minLevel) return;

    final old = minLevel;
    final msg =
        "Client log level changed: ${old.name} -> ${newLevel.name} (server '${value ?? ''}')";
    // When narrowing (e.g. → off) record under the OLD threshold first so the
    // announcement isn't dropped; when widening, set then record.
    if (newLevel.index > old.index) {
      log(msg, level: LogLevel.warn);
      minLevel = newLevel;
    } else {
      minLevel = newLevel;
      log(msg, level: LogLevel.warn);
    }
  }

  static String get logFilePath => _logFile?.path ?? 'Not initialized';

  static String tail({int lines = 100}) {
    if (_logFile == null || !_logFile!.existsSync()) return '(no logs)';
    final allLines = _logFile!.readAsLinesSync();
    return allLines.skip(allLines.length > lines ? allLines.length - lines : 0).join('\n');
  }
}
