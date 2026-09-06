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

class RecruitmentResumeAttachment {
  const RecruitmentResumeAttachment({
    required this.fileName,
    this.contentType = '',
    this.url = '',
    this.sizeBytes,
  });

  final String fileName;
  final String contentType;
  final String url;
  final int? sizeBytes;

  factory RecruitmentResumeAttachment.fromJson(Map<String, dynamic> json) {
    return RecruitmentResumeAttachment(
      fileName:
          (json['file_name'] as String?) ??
          (json['name'] as String?) ??
          (json['filename'] as String?) ??
          '简历附件',
      contentType:
          (json['content_type'] as String?) ??
          (json['mime_type'] as String?) ??
          '',
      url:
          (json['url'] as String?) ??
          (json['download_url'] as String?) ??
          (json['preview_url'] as String?) ??
          '',
      sizeBytes: _parseAttachmentSize(json['size_bytes'] ?? json['size']),
    );
  }
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
    this.resumeAttachments = const [],
    this.subject = '',
    this.body = '',
    this.position = '',
    this.lastAction = '',
    DateTime? receivedAt,
    this.updatedAt,
  }) : receivedAt = receivedAt ?? updatedAt;

  final String id;
  final String name;
  final String email;
  final String subject;
  final String body;
  final String position;
  final String stage;
  final String status;
  final bool hasResume;
  final bool hasCoverLetter;
  final List<RecruitmentResumeAttachment> resumeAttachments;
  final String lastAction;
  final DateTime? receivedAt;
  final DateTime? updatedAt;

  factory RecruitmentCandidate.fromJson(Map<String, dynamic> json) {
    return RecruitmentCandidate(
      id: json['id'] as String,
      name: json['name'] as String,
      email: json['email'] as String,
      subject: (json['subject'] as String?) ?? '',
      body: (json['body'] as String?) ?? '',
      position: (json['position'] as String?) ?? '',
      stage: json['stage'] as String,
      status: json['status'] as String,
      hasResume: (json['has_resume'] as bool?) ?? false,
      hasCoverLetter: (json['has_cover_letter'] as bool?) ?? false,
      resumeAttachments: _parseResumeAttachments(json),
      lastAction: (json['last_action'] as String?) ?? '',
      receivedAt: _parseRecruitmentTime(
        json['received_at'] ?? json['receivedAt'] ?? json['date'],
      ),
      updatedAt: _parseRecruitmentTime(
        json['updated_at'] ?? json['updatedAt'] ?? json['created_at'],
      ),
    );
  }
}

List<RecruitmentResumeAttachment> _parseResumeAttachments(
  Map<String, dynamic> json,
) {
  final raw = json['resume_attachments'] ?? json['attachments'];
  if (raw is! List) {
    return const [];
  }
  return raw
      .whereType<Map<String, dynamic>>()
      .map(RecruitmentResumeAttachment.fromJson)
      .where((attachment) => attachment.fileName.trim().isNotEmpty)
      .toList(growable: false);
}

int? _parseAttachmentSize(Object? value) {
  if (value is int) {
    return value;
  }
  if (value is num) {
    return value.toInt();
  }
  if (value is String) {
    return int.tryParse(value);
  }
  return null;
}

DateTime? _parseRecruitmentTime(Object? value) {
  if (value is int) {
    return DateTime.fromMillisecondsSinceEpoch(value, isUtc: true);
  }
  if (value is! String) {
    return null;
  }
  final text = value.trim();
  if (text.isEmpty) {
    return null;
  }
  return DateTime.tryParse(text) ??
      DateTime.tryParse(text.replaceFirst(' ', 'T'));
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

class RecruitmentResumeView {
  const RecruitmentResumeView({required this.url, required this.expiresAt});

  final String url;
  final DateTime? expiresAt;

  factory RecruitmentResumeView.fromJson(
    Map<String, dynamic> json,
    Uri baseUri,
  ) {
    final rawUrl = json['url'] as String?;
    if (rawUrl == null || rawUrl.trim().isEmpty) {
      throw const RecruitmentApiException('招聘服务返回为空');
    }
    final resolvedUrl = rawUrl.startsWith('/')
        ? baseUri.replace(
            path:
                '${baseUri.path.endsWith('/') ? baseUri.path.substring(0, baseUri.path.length - 1) : baseUri.path}$rawUrl',
          )
        : baseUri.resolve(rawUrl);
    return RecruitmentResumeView(
      url: resolvedUrl.toString(),
      expiresAt: _parseRecruitmentTime(json['expires_at']),
    );
  }
}

class RecruitmentInboxSyncResult {
  const RecruitmentInboxSyncResult({
    required this.syncId,
    required this.status,
    required this.mailbox,
    required this.folder,
    required this.scanned,
    required this.imported,
    required this.candidates,
  });

  final String syncId;
  final String status;
  final String mailbox;
  final String folder;
  final int scanned;
  final int imported;
  final List<RecruitmentCandidate> candidates;

  factory RecruitmentInboxSyncResult.fromJson(Map<String, dynamic> json) {
    return RecruitmentInboxSyncResult(
      syncId: json['sync_id'] as String,
      status: json['status'] as String,
      mailbox: json['mailbox'] as String,
      folder: json['folder'] as String,
      scanned: json['scanned'] as int,
      imported: json['imported'] as int,
      candidates: (json['candidates'] as List<dynamic>)
          .cast<Map<String, dynamic>>()
          .map(RecruitmentCandidate.fromJson)
          .toList(),
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

class RecruitmentProviderStatusComponent {
  const RecruitmentProviderStatusComponent({
    required this.name,
    required this.status,
    required this.message,
    this.version = '',
  });

  final String name;
  final String status;
  final String message;
  final String version;

  factory RecruitmentProviderStatusComponent.fromJson(
    Map<String, dynamic> json,
  ) {
    return RecruitmentProviderStatusComponent(
      name: (json['name'] as String?) ?? '',
      status: (json['status'] as String?) ?? 'unknown',
      message: (json['message'] as String?) ?? '',
      version: (json['version'] as String?) ?? '',
    );
  }
}

class RecruitmentProviderStatus {
  const RecruitmentProviderStatus({
    required this.status,
    required this.ready,
    required this.mailbox,
    required this.message,
    required this.components,
    this.checkedAt,
  });

  final String status;
  final bool ready;
  final String mailbox;
  final String message;
  final List<RecruitmentProviderStatusComponent> components;
  final DateTime? checkedAt;

  factory RecruitmentProviderStatus.fromJson(Map<String, dynamic> json) {
    return RecruitmentProviderStatus(
      status: (json['status'] as String?) ?? 'unknown',
      ready: (json['ready'] as bool?) ?? false,
      mailbox: (json['mailbox'] as String?) ?? '',
      message: (json['message'] as String?) ?? '',
      components: ((json['components'] as List<dynamic>?) ?? const [])
          .cast<Map<String, dynamic>>()
          .map(RecruitmentProviderStatusComponent.fromJson)
          .toList(growable: false),
      checkedAt: _parseRecruitmentTime(json['checked_at']),
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
    this.operator = '',
    this.accessToken = '',
    http.Client? httpClient,
  }) : _httpClient = httpClient ?? http.Client();

  final String baseUrl;
  final String operator;
  final String accessToken;
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

  Future<RecruitmentInboxSyncResult> syncInbox({
    required bool dryRun,
    String mailbox = 'hr@quanttide.com',
    String folder = 'INBOX',
    int pageSize = 50,
  }) async {
    final response = await _httpClient.post(
      _uri('/api/v1/recruitment/inbox/sync'),
      headers: _headers(write: true),
      body: jsonEncode({
        'mailbox': mailbox,
        'folder': folder,
        'page_size': pageSize,
        'dry_run': dryRun,
      }),
    );
    return RecruitmentInboxSyncResult.fromJson(
      _decodeResponse(response) as Map<String, dynamic>,
    );
  }

  Future<RecruitmentProviderStatus> checkProviderStatus() async {
    final response = await _httpClient.get(
      _uri('/api/v1/recruitment/provider/status'),
      headers: _headers(write: false),
    );
    return RecruitmentProviderStatus.fromJson(
      _decodeResponse(response) as Map<String, dynamic>,
    );
  }

  Future<RecruitmentCandidate> updateCandidateStatus({
    required String candidateId,
    required String status,
  }) async {
    final response = await _httpClient.patch(
      _uri('/api/v1/recruitment/candidates/$candidateId'),
      headers: _headers(write: true),
      body: jsonEncode({'status': status}),
    );
    return RecruitmentCandidate.fromJson(
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

  Future<RecruitmentResumeView> createResumeView({
    required String candidateId,
    required int attachmentIndex,
  }) async {
    final response = await _httpClient.post(
      _uri(
        '/api/v1/recruitment/candidates/$candidateId/resume/$attachmentIndex/view',
      ),
      headers: _headers(write: false),
      body: jsonEncode(<String, Object?>{}),
    );
    final decoded = _decodeResponse(response);
    if (decoded is! Map<String, dynamic>) {
      throw const RecruitmentApiException('招聘服务返回为空');
    }
    return RecruitmentResumeView.fromJson(decoded, Uri.parse(baseUrl));
  }

  Uri _uri(String path) {
    final base = Uri.parse(baseUrl);
    final basePath = base.path.endsWith('/')
        ? base.path.substring(0, base.path.length - 1)
        : base.path;
    final relativePath = path.startsWith('/') ? path.substring(1) : path;
    return base.replace(path: '$basePath/$relativePath');
  }

  Map<String, String> _headers({required bool write}) {
    final headers = <String, String>{'Content-Type': 'application/json'};
    if (accessToken.trim().isNotEmpty) {
      headers['Authorization'] = 'Bearer ${accessToken.trim()}';
    } else if (operator.trim().isNotEmpty) {
      headers['X-Operator'] = operator;
    }
    if (write && accessToken.trim().isEmpty) {
      headers['X-Recruitment-Permission'] = 'write';
    }
    return headers;
  }

  Object _decodeResponse(http.Response response) {
    final Object? body;
    try {
      body = response.body.isEmpty ? null : jsonDecode(response.body);
    } on FormatException {
      throw const RecruitmentApiException('招聘服务暂不可用，请稍后重试');
    }
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
