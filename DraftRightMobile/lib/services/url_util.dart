/// Trims trailing slashes from a backend URL so a `base + '/path'`
/// concatenation never produces a double slash. Single source of truth
/// (#205 #6) — this was previously reimplemented ~7 times in two styles
/// (a `while (endsWith('/'))` loop and `replaceAll(RegExp(r'/+$'), '')`).
String normalizeBackendUrl(String url) {
  var u = url;
  while (u.endsWith('/')) {
    u = u.substring(0, u.length - 1);
  }
  return u;
}
