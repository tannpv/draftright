// Unit test for BackendClient.redeemAppleTransaction — the client-side call
// that hands StoreKit's signed transaction to the backend for verification +
// Pro grant (POST /payment/apple/redeem).
//
// Uses a MockClient (package:http/testing.dart) rather than the
// `_FakeBackend extends BackendClient` override pattern used elsewhere
// (e.g. payment_plan_resolution_test.dart) because the point of this test is
// to prove the exact wire contract — method, path, JSON body shape, and
// Bearer header — not just that the method got called.
//
// AuthService.getAccessToken() throws unless a token was stored via
// login/register/social-login (which hit secure storage + platform
// channels), so _StubAuth overrides just that one method to return a fixed
// token without touching any plugin.

import 'dart:convert';

import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';

import 'package:draftright_mobile/services/auth_service.dart';
import 'package:draftright_mobile/services/backend_client.dart';

class _StubAuth extends AuthService {
  @override
  Future<String> getAccessToken() async => 'jwt';
}

void main() {
  test(
      'redeemAppleTransaction posts the signed transaction to /payment/apple/redeem',
      () async {
    late http.Request seen;
    final client = MockClient((req) async {
      seen = req;
      return http.Response('{}', 200);
    });
    final backend = BackendClient(
      auth: _StubAuth(),
      getBaseUrl: () => 'https://api.test',
      httpClient: client,
    );

    await backend.redeemAppleTransaction('signed-jws-1');

    expect(seen.method, 'POST');
    expect(seen.url.path, '/payment/apple/redeem');
    expect(jsonDecode(seen.body), {'signedTransaction': 'signed-jws-1'});
    expect(seen.headers['authorization'], 'Bearer jwt');
  });
}
