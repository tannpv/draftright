// Unit tests for ScreenshotCompressor — the pre-upload downscale/recompress
// step that keeps bug-report screenshots well under the backend's 5 MB body
// cap. Pure Dart (image package + dart:io temp files), so it runs under
// `flutter test` with no device or plugins.
//
// Regression guard for issue #68: a near-5 MB image passed the client's
// image-file size guard but 413'd at the server (whose limit is on the whole
// multipart body). Compressing first makes that impossible.

import 'dart:io';

import 'package:flutter_test/flutter_test.dart';
import 'package:image/image.dart' as img;

import 'package:draftright_mobile/services/screenshot_compressor.dart';

void main() {
  late Directory tmp;

  setUp(() {
    tmp = Directory.systemTemp.createTempSync('dr_compressor_test_');
  });

  tearDown(() {
    if (tmp.existsSync()) tmp.deleteSync(recursive: true);
  });

  /// Writes an [w]x[h] noise PNG (incompressible → large file) to disk.
  File writeNoisePng(int w, int h) {
    final image = img.Image(width: w, height: h);
    var seed = 1;
    for (final p in image) {
      // Deterministic LCG so the file is reproducible but incompressible.
      seed = (seed * 1103515245 + 12345) & 0x7fffffff;
      p
        ..r = seed & 0xff
        ..g = (seed >> 8) & 0xff
        ..b = (seed >> 16) & 0xff;
    }
    final f = File('${tmp.path}/src_${w}x$h.png')
      ..writeAsBytesSync(img.encodePng(image));
    return f;
  }

  test('downscales a large image so its longest edge is <= maxEdge', () async {
    final src = writeNoisePng(3000, 2000);

    final out = await ScreenshotCompressor.compressForUpload(src,
        outputDir: tmp);

    final decoded = img.decodeImage(out.readAsBytesSync())!;
    expect(decoded.width, ScreenshotCompressor.maxEdge); // 3000 -> 1600
    expect(decoded.height, lessThanOrEqualTo(ScreenshotCompressor.maxEdge));
    // Aspect ratio preserved (2000/3000 * 1600 = 1066±1).
    expect(decoded.height, closeTo(1066, 2));
  });

  test('output is a JPEG file, comfortably under the 5 MB server cap',
      () async {
    final src = writeNoisePng(4000, 4000); // > 5 MB PNG of noise

    final out = await ScreenshotCompressor.compressForUpload(src,
        outputDir: tmp);

    expect(out.path, endsWith('.jpg'));
    expect(out.lengthSync(), lessThan(5 * 1024 * 1024));
    // Must still be a valid, decodable image.
    expect(img.decodeImage(out.readAsBytesSync()), isNotNull);
  });

  test('does not upscale an already-small image', () async {
    final src = writeNoisePng(800, 600);

    final out = await ScreenshotCompressor.compressForUpload(src,
        outputDir: tmp);

    final decoded = img.decodeImage(out.readAsBytesSync())!;
    expect(decoded.width, 800);
    expect(decoded.height, 600);
  });

  test('returns the original file when bytes are not a decodable image',
      () async {
    final notAnImage = File('${tmp.path}/garbage.bin')
      ..writeAsBytesSync(List<int>.generate(1024, (i) => i % 256));

    final out = await ScreenshotCompressor.compressForUpload(notAnImage,
        outputDir: tmp);

    expect(out.path, notAnImage.path);
  });
}
