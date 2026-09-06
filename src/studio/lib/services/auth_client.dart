import 'dart:convert';

import 'package:http/http.dart' as http;

class AuthClientException implements Exception {
  const AuthClientException(this.message);

  final String message;

  @override
  String toString() => message;
}

class AuthClient {
  AuthClient({required this.baseUrl, http.Client? httpClient})
    : _httpClient = httpClient ?? http.Client();

  final String baseUrl;
  final http.Client _httpClient;

  Future<String> login({
    required String username,
    required String password,
  }) async {
    final response = await _httpClient.post(
      _uri('/oauth/token'),
      headers: const {
        'Content-Type': 'application/x-www-form-urlencoded',
        'Accept': 'application/json',
      },
      body: Uri(
        queryParameters: {
          'grant_type': 'password',
          'username': username,
          'password': password,
        },
      ).query,
    );

    Object? body;
    try {
      body = response.body.isEmpty ? null : jsonDecode(response.body);
    } on FormatException {
      throw const AuthClientException('认证服务返回无效响应');
    }
    if (response.statusCode >= 200 && response.statusCode < 300) {
      if (body is Map<String, dynamic> && body['access_token'] is String) {
        final token = (body['access_token'] as String).trim();
        if (token.isNotEmpty) {
          return token;
        }
      }
      throw const AuthClientException('认证服务未返回访问令牌');
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
