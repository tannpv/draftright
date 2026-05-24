import 'dart:io';
import 'package:crypto/crypto.dart';
import 'package:flutter/services.dart';
import 'package:http/http.dart' as http;

/// Thrown when a downloaded language pack's SHA-256 does not match the manifest.
/// A mismatch means the bytes are corrupt or tampered with — never installed.
class PackIntegrityError implements Exception {
  final String message;
  PackIntegrityError(this.message);
  @override
  String toString() => 'PackIntegrityError: $message';
}

/// Thrown when the pack server returns a non-200 response.
class PackDownloadError implements Exception {
  final int statusCode;
  PackDownloadError(this.statusCode);
  @override
  String toString() => 'PackDownloadError: HTTP $statusCode';
}

/// Install/verify/remove surface for downloadable language packs. The settings
/// UI depends on this interface (not the concrete service) so it can be tested
/// against a fake.
abstract class PackInstaller {
  Future<bool> isInstalled(String packId);
  Future<String> install({
    required String packId,
    required String url,
    required String sha256,
    int? sizeBytes,
    void Function(double progress)? onProgress,
  });
  Future<void> remove(String packId);
}

/// Downloads, verifies and installs downloadable IME language packs (Japanese,
/// then Korean/Chinese) into a shared directory the keyboard extension can read.
///
/// The pack is streamed to a `<id>.pack.part` temp file while its SHA-256 is
/// computed. Only after the hash matches the manifest is the temp file moved
/// atomically into place, so a failed or corrupt download never leaves a usable
/// pack behind. [baseDir] is the App Group container (iOS) / files dir
/// (Android); tests pass a temporary directory.
class ImePackService implements PackInstaller {
  ImePackService({required Directory baseDir, http.Client? httpClient})
      : _baseDir = baseDir,
        _http = httpClient ?? http.Client();

  final Directory _baseDir;
  final http.Client _http;

  // Reuses the existing App Group channel (iOS) / share channel (Android),
  // which both answer `sharedPackDir` with a directory the keyboard can read.
  static const MethodChannel _iosChannel =
      MethodChannel('com.draftright.v2/app_group');
  static const MethodChannel _androidChannel = MethodChannel('draftright/share');

  /// Builds a service rooted at the platform's shared directory (App Group
  /// container on iOS, app files dir on Android) so installed packs are visible
  /// to the keyboard extension. Tests construct the service directly instead.
  static Future<ImePackService> forPlatform({http.Client? httpClient}) async {
    final channel = Platform.isIOS ? _iosChannel : _androidChannel;
    final dir = await channel.invokeMethod<String>('sharedPackDir');
    if (dir == null || dir.isEmpty) {
      throw StateError('shared pack directory unavailable on this platform');
    }
    return ImePackService(baseDir: Directory(dir), httpClient: httpClient);
  }

  Directory get _packsDir => Directory('${_baseDir.path}/packs');

  /// Final on-disk path for an installed pack.
  String packPath(String packId) => '${_packsDir.path}/$packId.pack';

  /// Whether the pack is installed and ready for the keyboard to mmap.
  @override
  Future<bool> isInstalled(String packId) => File(packPath(packId)).exists();

  /// Streams [url] to a temp file, verifies its SHA-256 equals [sha256], then
  /// atomically installs it. Returns the final path. [onProgress] (0.0–1.0) is
  /// called as bytes arrive when the total size is known.
  @override
  Future<String> install({
    required String packId,
    required String url,
    required String sha256,
    int? sizeBytes,
    void Function(double progress)? onProgress,
  }) async {
    await _packsDir.create(recursive: true);
    final tempFile = File('${packPath(packId)}.part');
    if (await tempFile.exists()) await tempFile.delete();

    final response = await _http.send(http.Request('GET', Uri.parse(url)));
    if (response.statusCode != 200) {
      throw PackDownloadError(response.statusCode);
    }

    final total = sizeBytes ?? response.contentLength ?? 0;
    final digestSink = _Sha256Sink();
    final sink = tempFile.openWrite();
    var received = 0;
    try {
      await for (final chunk in response.stream) {
        sink.add(chunk);
        digestSink.add(chunk);
        received += chunk.length;
        if (onProgress != null && total > 0) {
          onProgress((received / total).clamp(0.0, 1.0));
        }
      }
      await sink.flush();
    } finally {
      await sink.close();
    }

    final actual = digestSink.digest().toString();
    if (actual != sha256) {
      if (await tempFile.exists()) await tempFile.delete();
      throw PackIntegrityError(
          'pack "$packId" hash mismatch: expected $sha256, got $actual');
    }

    final finalFile = File(packPath(packId));
    if (await finalFile.exists()) await finalFile.delete();
    await tempFile.rename(finalFile.path);
    onProgress?.call(1.0);
    return finalFile.path;
  }

  /// Deletes an installed pack and frees its disk space.
  @override
  Future<void> remove(String packId) async {
    final f = File(packPath(packId));
    if (await f.exists()) await f.delete();
  }
}

/// Incremental SHA-256 over streamed chunks, so an 18 MB pack is never held in
/// memory twice.
class _Sha256Sink {
  final _output = _DigestCollector();
  late final Sink<List<int>> _input = sha256.startChunkedConversion(_output);

  void add(List<int> chunk) => _input.add(chunk);

  Digest digest() {
    _input.close();
    return _output.value;
  }
}

class _DigestCollector implements Sink<Digest> {
  late Digest value;
  @override
  void add(Digest data) => value = data;
  @override
  void close() {}
}
