// TC: EXTTOK-013
// TC: EXTTOK-011
// TC: EXTTOK-012
import 'package:flutter_test/flutter_test.dart';
import 'package:shared_preferences/shared_preferences.dart';
import 'package:draftright_mobile/services/extension_token_service.dart';

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();

  setUp(() async {
    SharedPreferences.setMockInitialValues({});
  });

  test('deviceId is generated once and persisted', () async {
    final svc = ExtensionTokenService(baseUrl: 'http://localhost:3000');
    final first = await svc.deviceId();
    final second = await svc.deviceId();
    expect(first, equals(second));
    expect(
      first,
      matches(RegExp(
        r'^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$',
      )),
    );
  });

  test('storeToken writes to SharedPreferences with flutter. prefix', () async {
    final svc = ExtensionTokenService(baseUrl: 'http://localhost:3000');
    await svc.storeToken('dr_ext_abc');
    final prefs = await SharedPreferences.getInstance();
    // SharedPreferences plugin auto-prefixes 'flutter.' to key writes,
    // so the Android extension reads `flutter.draftright.extensionToken`.
    expect(prefs.getString('draftright.extensionToken'), 'dr_ext_abc');
  });

  test('clearToken removes the token from SharedPreferences', () async {
    final svc = ExtensionTokenService(baseUrl: 'http://localhost:3000');
    await svc.storeToken('dr_ext_abc');
    await svc.clearToken();
    final prefs = await SharedPreferences.getInstance();
    expect(prefs.getString('draftright.extensionToken'), isNull);
  });
}
