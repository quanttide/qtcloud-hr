import 'dart:convert';

import 'package:http/http.dart' as http;

class AuthClientException implements Exception {
  const AuthClientException(this.message);

  final String message;

  @override
  String toString() => message;
}

class AuthTokens {
  const AuthTokens({required this.accessToken, required this.refreshToken});

  final String accessToken;
  final String refreshToken;
}

class AuthClient {
  AuthClient({required this.baseUrl, http.Client? httpClient})
    : _httpClient = httpClient ?? http.Client();

  final String baseUrl;
  final http.Client _httpClient;

  Future<AuthTokens> login({
    required String username,
    required String password,
  }) async {
    return _requestToken({
      'grant_type': 'password',
      'username': username,
      'password': password,
    });
  }

  Future<AuthTokens> refresh({required String refreshToken}) {
    return _requestToken({
      'grant_type': 'refresh_token',
      'refresh_token': refreshToken,
    });
  }

  Future<AuthTokens> _requestToken(Map<String, String> parameters) async {
    final response = await _httpClient.post(
      _uri('/oauth/token'),
      headers: const {
        'Content-Type': 'application/x-www-form-urlencoded',
        'Accept': 'application/json',
      },
      body: Uri(queryParameters: parameters).query,
    );
    Object? body;
    try {
      body = response.body.isEmpty ? null : jsonDecode(response.body);
    } on FormatException {
      throw const AuthClientException('认证服务返回无效响应');
    }
    if (response.statusCode >= 200 && response.statusCode < 300) {
      if (body is Map<String, dynamic>) {
        final accessToken = (body['access_token'] as String?)?.trim() ?? '';
        final refreshToken = (body['refresh_token'] as String?)?.trim() ?? '';
        if (accessToken.isNotEmpty && refreshToken.isNotEmpty) {
          return AuthTokens(
            accessToken: accessToken,
            refreshToken: refreshToken,
          );
        }
      }
      throw const AuthClientException('认证服务未返回完整令牌');
    }
    if (body is Map<String, dynamic>) {
      final description = body['error_description'];
      if (description is String && description.isNotEmpty) {
        throw AuthClientException(description);
      }
      final error = body['error'];
      if (error is String && error.isNotEmpty) {
        throw AuthClientException(error);
      }
    }
    throw const AuthClientException('账号或密码错误');
  }

  Uri _uri(String path) {
    final base = Uri.parse(baseUrl);
    final basePath = base.path.endsWith('/')
        ? base.path.substring(0, base.path.length - 1)
        : base.path;
    final relativePath = path.startsWith('/') ? path.substring(1) : path;
    return base.replace(path: '$basePath/$relativePath');
  }
}
