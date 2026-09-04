import 'dart:convert';

import 'package:http/http.dart' as http;

enum RecruitmentAction {
  sendSurvey('send_survey'),
  sendTrainingInvite('send_training_invite'),
  sendExam('send_exam'),
  createInterviewNotice('create_interview_notice');

  const RecruitmentAction(this.value);

  final String value;
}

class RecruitmentCandidate {
  const RecruitmentCandidate({
    required this.id,
    required this.name,
    required this.email,
    required this.stage,
    required this.status,
    required this.hasResume,
    required this.hasCoverLetter,
    this.position = '',
    this.lastAction = '',
  });

  final String id;
  final String name;
  final String email;
  final String position;
  final String stage;
  final String status;
  final bool hasResume;
  final bool hasCoverLetter;
  final String lastAction;

  factory RecruitmentCandidate.fromJson(Map<String, dynamic> json) {
    return RecruitmentCandidate(
      id: json['id'] as String,
      name: json['name'] as String,
      email: json['email'] as String,
      position: (json['position'] as String?) ?? '',
      stage: json['stage'] as String,
      status: json['status'] as String,
      hasResume: (json['has_resume'] as bool?) ?? false,
      hasCoverLetter: (json['has_cover_letter'] as bool?) ?? false,
      lastAction: (json['last_action'] as String?) ?? '',
    );
  }
}

class RecruitmentReport {
  const RecruitmentReport({
    required this.reportId,
    required this.status,
    required this.markdown,
  });

  final String reportId;
  final String status;
  final String markdown;

  factory RecruitmentReport.fromJson(Map<String, dynamic> json) {
    return RecruitmentReport(
      reportId: json['report_id'] as String,
      status: json['status'] as String,
      markdown: json['markdown'] as String,
    );
  }
}

class RecruitmentActionResult {
  const RecruitmentActionResult({
    required this.actionId,
    required this.candidateId,
    required this.action,
    required this.status,
    required this.message,
  });

  final String actionId;
  final String candidateId;
  final String action;
  final String status;
  final String message;

  factory RecruitmentActionResult.fromJson(Map<String, dynamic> json) {
    return RecruitmentActionResult(
      actionId: json['action_id'] as String,
      candidateId: json['candidate_id'] as String,
      action: json['action'] as String,
      status: json['status'] as String,
      message: json['message'] as String,
    );
  }
}

class RecruitmentApiException implements Exception {
  const RecruitmentApiException(this.message);

  final String message;

  @override
  String toString() => message;
}

class RecruitmentApiClient {
  RecruitmentApiClient({
    required this.baseUrl,
    required this.operator,
    http.Client? httpClient,
  }) : _httpClient = httpClient ?? http.Client();

  final String baseUrl;
  final String operator;
  final http.Client _httpClient;

  Future<List<RecruitmentCandidate>> listCandidates() async {
    final response = await _httpClient.get(
      _uri('/api/v1/recruitment/candidates'),
      headers: _headers(write: false),
    );
    final decoded = _decodeResponse(response);
    return (decoded as List<dynamic>)
        .cast<Map<String, dynamic>>()
        .map(RecruitmentCandidate.fromJson)
        .toList();
  }

  Future<RecruitmentReport> createReport({
    required int days,
    required bool dryRun,
  }) async {
    final response = await _httpClient.post(
      _uri('/api/v1/recruitment/reports'),
      headers: _headers(write: false),
      body: jsonEncode({'days': days, 'dry_run': dryRun}),
    );
    return RecruitmentReport.fromJson(
      _decodeResponse(response) as Map<String, dynamic>,
    );
  }

  Future<RecruitmentActionResult> runAction({
    required String candidateId,
    required RecruitmentAction action,
    required bool dryRun,
    required Map<String, Object?> params,
  }) async {
    final response = await _httpClient.post(
      _uri('/api/v1/recruitment/candidates/$candidateId/actions'),
      headers: _headers(write: true),
      body: jsonEncode({
        'action': action.value,
        'dry_run': dryRun,
        'params': params,
      }),
    );
    return RecruitmentActionResult.fromJson(
      _decodeResponse(response) as Map<String, dynamic>,
    );
  }

  Uri _uri(String path) => Uri.parse(baseUrl).resolve(path);

  Map<String, String> _headers({required bool write}) {
    final headers = <String, String>{
      'Content-Type': 'application/json',
      'X-Operator': operator,
    };
    if (write) {
      headers['X-Recruitment-Permission'] = 'write';
    }
    return headers;
  }

  Object _decodeResponse(http.Response response) {
    final body = response.body.isEmpty ? null : jsonDecode(response.body);
    if (response.statusCode >= 200 && response.statusCode < 300) {
      if (body == null) {
        throw const RecruitmentApiException('招聘服务返回为空');
      }
      return body;
    }
    if (body is Map<String, dynamic>) {
      final error = body['error'];
      if (error is String && error.isNotEmpty) {
        throw RecruitmentApiException(error);
      }
      if (error is Map<String, dynamic>) {
        final message = error['message'];
        if (message is String && message.isNotEmpty) {
          throw RecruitmentApiException(message);
        }
      }
    }
    throw const RecruitmentApiException('招聘服务暂不可用，请稍后重试');
  }
}
