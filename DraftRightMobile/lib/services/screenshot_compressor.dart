import 'dart:io';

import 'package:image/image.dart' as img;
import 'package:path_provider/path_provider.dart';

import 'package:draftright_mobile/services/logger_service.dart';

/// Downscales and recompresses a picked screenshot before it is uploaded with
/// a bug report.
///
/// Why this exists (issue #68): the client used to send the raw picked file
/// and only guarded its *image-file* size against 5 MB. The backend's limit is
/// on the whole *multipart body* (image + fields + boundaries), so an image
/// near 5 MB slipped past the client and 413'd at the server — and the failure
/// was invisible. Recompressing to a bounded JPEG keeps the upload at a few
/// hundred KB, which no realistic screenshot exceeds. Mirrors the admin portal
/// fix (downscale ≤1600px + JPEG q0.85).
class ScreenshotCompressor {
  /// Longest-edge bound. A screenshot is legible well below this; keeping it
  /// small is what guarantees the body stays under the server cap.
  static const int maxEdge = 1600;

  /// JPEG quality. 0.85 is visually lossless for UI screenshots at a fraction
  /// of the PNG size.
  static const int jpegQuality = 85;

  /// Returns a NEW temp JPEG file: the source decoded, EXIF-orientation baked,
  /// downscaled so its longest edge is ≤ [maxEdge] (never upscaled), and
  /// re-encoded at [quality].
  ///
  /// If [src] cannot be decoded as an image, [src] is returned unchanged so
  /// the backend — the source of truth on accepted formats — can reject it
  /// with a message the UI now surfaces. Never throws for a decode failure.
  ///
  /// [outputDir] overrides where the temp file is written (tests pass a temp
  /// dir); production uses the system temp directory.
  static Future<File> compressForUpload(
    File src, {
    int maxEdge = maxEdge,
    int quality = jpegQuality,
    Directory? outputDir,
  }) async {
    try {
      final decoded = img.decodeImage(await src.readAsBytes());
      if (decoded == null) {
        DRLogger.warn(
          'Screenshot not decodable — uploading original for backend to validate',
          category: 'BUG_REPORT',
        );
        return src;
      }

      // Respect the camera/EXIF orientation before resizing so portrait shots
      // don't upload sideways.
      final oriented = img.bakeOrientation(decoded);

      final longest =
          oriented.width >= oriented.height ? oriented.width : oriented.height;
      final resized = longest > maxEdge
          ? img.copyResize(
              oriented,
              width: oriented.width >= oriented.height ? maxEdge : null,
              height: oriented.height > oriented.width ? maxEdge : null,
            )
          : oriented;

      final jpg = img.encodeJpg(resized, quality: quality);

      // On device use the app's temp dir (always writable); tests pass their
      // own dir. Directory.systemTemp is unreliable on Android — a failed
      // write there would drop us back to the raw file and re-introduce the
      // 413 this whole class exists to prevent (issue #68).
      final dir = outputDir ?? await getTemporaryDirectory();
      final out = File(
          '${dir.path}/dr_bugshot_${DateTime.now().microsecondsSinceEpoch}.jpg');
      await out.writeAsBytes(jpg, flush: true);
      return out;
    } catch (e) {
      // Any unexpected failure: fall back to the original file rather than
      // blocking the report. The backend + the (now visible) error banner are
      // the backstop.
      DRLogger.warn('Screenshot compression failed, using original: $e',
          category: 'BUG_REPORT');
      return src;
    }
  }
}
