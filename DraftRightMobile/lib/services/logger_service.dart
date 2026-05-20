import 'dart:io';
import 'package:path_provider/path_provider.dart';

/// Log severity. Lines are tagged `[LEVEL]` so failures are skimmable and
/// greppable (`grep "\[ERROR\]"`) instead of buried in same-level category
/// chatter. [warn] = something degraded but handled; [error] = an operation
/// failed.
enum LogLevel { info, warn, error }

/// Centralized file logger for DraftRight.
class DRLogger {
  static File? _logFile;

  static Future<void> init() async {
    final dir = await getApplicationDocumentsDirectory();
    final logDir = Directory('${dir.path}/logs');
    if (!logDir.existsSync()) logDir.createSync(recursive: true);
    _logFile = File('${logDir.path}/draftright.log');
  }

  static void log(String message,
      {String category = 'APP', LogLevel level = LogLevel.info}) {
    final timestamp = DateTime.now().toIso8601String();
    // Pad the level so the [CATEGORY] column stays aligned across lines.
    final levelTag = level.name.toUpperCase().padRight(5);
    final line = '[$timestamp] [$levelTag] [$category] $message';
    _logFile?.writeAsStringSync('$line\n', mode: FileMode.append);
    assert(() { print(line); return true; }());
  }

  /// Log a handled-but-degraded condition at [LogLevel.warn].
  static void warn(String message, {String category = 'APP'}) =>
      log(message, category: category, level: LogLevel.warn);

  /// Log a failed operation at [LogLevel.error].
  static void error(String message, {String category = 'APP'}) =>
      log(message, category: category, level: LogLevel.error);

  static String get logFilePath => _logFile?.path ?? 'Not initialized';

  static String tail({int lines = 100}) {
    if (_logFile == null || !_logFile!.existsSync()) return '(no logs)';
    final allLines = _logFile!.readAsLinesSync();
    return allLines.skip(allLines.length > lines ? allLines.length - lines : 0).join('\n');
  }
}
