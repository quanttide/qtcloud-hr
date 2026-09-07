import 'dart:convert';

import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:qtcloud_hr_studio/services/auth_client.dart';

void main() {
  test('password login posts form data and returns both tokens', () async {
    final client = AuthClient(
      baseUrl: 'https://api.example.test/qtcloud-auth',
      httpClient: MockClient((request) async {
        expect(request.url.path, '/qtcloud-auth/oauth/token');
        expect(
          request.headers['content-type'],
          'application/x-www-form-urlencoded',
        );
        expect(request.body, contains('grant_type=password'));
        expect(request.body, contains('username=user%40example.com'));
        return jsonResponse({
          'access_token': 'access-token',
          'token_type': 'Bearer',
          'expires_in': 3600,
          'refresh_token': 'refresh-token',
        });
      }),
    );

    final tokens = await client.login(
      username: 'user@example.com',
      password: 'password',
    );
    expect(tokens.accessToken, 'access-token');
    expect(tokens.refreshToken, 'refresh-token');
  });

  test('refresh posts refresh grant and returns rotated tokens', () async {
    final client = AuthClient(
      baseUrl: 'https://api.example.test/qtcloud-auth',
      httpClient: MockClient((request) async {
        expect(request.url.path, '/qtcloud-auth/oauth/token');
        expect(request.body, contains('grant_type=refresh_token'));
        expect(request.body, contains('refresh_token=old-refresh-token'));
        return jsonResponse({
          'access_token': 'new-access-token',
          'token_type': 'Bearer',
          'expires_in': 3600,
          'refresh_token': 'new-refresh-token',
        });
      }),
    );

    final token = await client.refresh(refreshToken: 'old-refresh-token');

    expect(token.accessToken, 'new-access-token');
    expect(token.refreshToken, 'new-refresh-token');
  });
}

http.Response jsonResponse(Object body) {
  return http.Response(
    jsonEncode(body),
    200,
    headers: {'content-type': 'application/json; charset=utf-8'},
  );
}
