import 'dart:io';
import 'package:path_provider/path_provider.dart';

/// Centralized file logger for DraftRight.
class DRLogger {
  static File? _logFile;

  static Future<void> init() async {
    final dir = await getApplicationDocumentsDirectory();
    final logDir = Directory('${dir.path}/logs');
    if (!logDir.existsSync()) logDir.createSync(recursive: true);
    _logFile = File('${logDir.path}/draftright.log');
  }

  static void log(String message, {String category = 'APP'}) {
    final timestamp = DateTime.now().toIso8601String();
    final line = '[$timestamp] [$category] $message';
    _logFile?.writeAsStringSync('$line\n', mode: FileMode.append);
    assert(() { print(line); return true; }());
  }

  static String get logFilePath => _logFile?.path ?? 'Not initialized';

  static String tail({int lines = 100}) {
    if (_logFile == null || !_logFile!.existsSync()) return '(no logs)';
    final allLines = _logFile!.readAsLinesSync();
    return allLines.skip(allLines.length > lines ? allLines.length - lines : 0).join('\n');
  }
}
