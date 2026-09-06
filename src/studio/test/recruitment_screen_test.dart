import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:qtcloud_hr_studio/screens/recruitment_screen.dart';
import 'package:qtcloud_hr_studio/services/recruitment_api_client.dart';

void main() {
  List<Map<String, Object?>> candidatesJson() => [
    {
      'id': 'cand_old',
      'name': '旧候选人',
      'email': 'old@example.com',
      'subject': '旧候选人投递',
      'body': '',
      'position': '产品经理',
      'stage': 'new',
      'status': 'pending',
      'has_resume': true,
      'has_cover_letter': false,
      'received_at': '2026-09-03T09:00:00Z',
      'updated_at': '2026-09-03T09:00:00Z',
    },
    {
      'id': 'cand_new',
      'name': '新候选人',
      'email': 'new@example.com',
      'subject': '新候选人投递',
      'body': 'HR 您好，我想投递产品经理岗位，附件是我的简历。',
      'position': '产品经理',
      'stage': 'new',
      'status': 'pending',
      'has_resume': true,
      'has_cover_letter': true,
      'resume_attachments': [
        {
          'file_name': '新候选人-产品经理简历.pdf',
          'content_type': 'application/pdf',
          'url': 'https://files.example.test/resumes/cand_new.pdf',
        },
      ],
      'received_at': '2026-09-04T10:00:00Z',
      'updated_at': '2026-09-04T10:00:00Z',
    },
    {
      'id': 'cand_processed',
      'name': '已处理候选人',
      'email': 'processed@example.com',
      'subject': '已处理候选人投递',
      'body': 'HR 您好，我有正文并已进入问卷流程。',
      'position': '运营',
      'stage': 'survey_sent',
      'status': 'passed',
      'has_resume': true,
      'has_cover_letter': true,
      'last_action': 'send_survey',
      'received_at': '2026-09-02T08:00:00Z',
      'updated_at': '2026-09-02T08:00:00Z',
    },
  ];

  RecruitmentApiClient clientForCandidates(
    List<Map<String, Object?>> candidates,
  ) {
    return RecruitmentApiClient(
      baseUrl: 'https://api.example.test',
      operator: 'tester',
      httpClient: MockClient((request) async {
        if (request.method == 'POST' &&
            request.url.path.contains('/resume/') &&
            request.url.path.endsWith('/view')) {
          return http.Response.bytes(
            utf8.encode(
              jsonEncode({
                'url': '/api/v1/recruitment/resume-view/token_001',
                'expires_at': '2026-09-05T06:00:00Z',
              }),
            ),
            201,
            headers: {'content-type': 'application/json; charset=utf-8'},
          );
        }
        return http.Response.bytes(
          utf8.encode(jsonEncode(candidates)),
          200,
          headers: {'content-type': 'application/json; charset=utf-8'},
        );
      }),
    );
  }

  test('Email.fromCandidate keeps provider subject and body', () {
    const candidate = RecruitmentCandidate(
      id: 'cand_001',
      name: '张三',
      email: 'zhangsan@example.com',
      subject: '应聘后端开发',
      body: 'HR 您好，我想投递后端开发岗位，附件是我的简历。',
      stage: 'new',
      status: 'pending',
      hasResume: true,
      hasCoverLetter: true,
      resumeAttachments: [
        RecruitmentResumeAttachment(
          fileName: '张三-后端开发简历.pdf',
          contentType: 'application/pdf',
          url: 'https://files.example.test/resumes/cand_001.pdf',
        ),
      ],
    );

    final email = Email.fromCandidate(candidate, 0);

    expect(email.subject, '应聘后端开发');
    expect(email.body, contains('投递后端开发岗位'));
    expect(email.hasBody, isTrue);
    expect(email.resumeAttachments.single.fileName, '张三-后端开发简历.pdf');
  });

  test('Email.fromCandidate does not use garbled candidate names', () {
    const candidate = RecruitmentCandidate(
      id: 'cand_002',
      name: '����',
      email: 'candidate@example.com',
      subject: '[产品经理] - [王五] - [浙江越秀外国语学院] - [3个月以上]',
      body: '您好，希望投递产品经理岗位。',
      stage: 'new',
      status: 'pending',
      hasResume: true,
      hasCoverLetter: true,
    );

    final email = Email.fromCandidate(candidate, 0);

    expect(email.name, '王五');
  });

  test(
    'Email.fromCandidate preserves explicit pass for resume-only candidate',
    () {
      const candidate = RecruitmentCandidate(
        id: 'cand_003',
        name: '李四',
        email: 'lisi@example.com',
        subject: '应聘数据工程师',
        body: '',
        stage: 'new',
        status: 'passed',
        hasResume: true,
        hasCoverLetter: false,
      );

      final email = Email.fromCandidate(candidate, 0);

      expect(email.detected, DetectResult.only);
      expect(email.status, EmailStatus.passed);
    },
  );

  testWidgets('Recruitment page shows selected email body when available', (
    tester,
  ) async {
    final client = clientForCandidates([
      {
        'id': 'cand_001',
        'name': '张三',
        'email': 'zhangsan@example.com',
        'subject': '应聘后端开发',
        'body': 'HR 您好，我想投递后端开发岗位，附件是我的简历。',
        'position': '后端开发',
        'stage': 'new',
        'status': 'pending',
        'has_resume': true,
        'has_cover_letter': true,
        'resume_attachments': [
          {
            'file_name': '张三-后端开发简历.docx',
            'content_type':
                'application/vnd.openxmlformats-officedocument.wordprocessingml.document',
            'url': 'https://files.example.test/resumes/cand_001.docx',
          },
        ],
        'updated_at': '2026-09-04T00:00:00Z',
      },
    ]);

    await tester.pumpWidget(
      MaterialApp(home: RecruitmentPage(apiClient: client)),
    );
    await tester.pumpAndSettle();

    expect(find.text('邮件正文'), findsOneWidget);
    expect(find.textContaining('投递后端开发岗位'), findsOneWidget);
    expect(find.text('简历附件'), findsOneWidget);
    expect(find.text('张三-后端开发简历.docx'), findsOneWidget);
    expect(find.text('简历预览'), findsNothing);
  });

  testWidgets('Recruitment page previews resume only after manual request', (
    tester,
  ) async {
    var requestedResumeView = false;
    final client = RecruitmentApiClient(
      baseUrl: 'https://api.example.test',
      operator: 'tester',
      httpClient: MockClient((request) async {
        if (request.method == 'GET' &&
            request.url.path == '/api/v1/recruitment/candidates') {
          return http.Response.bytes(
            utf8.encode(
              jsonEncode([
                {
                  'id': 'cand_001',
                  'name': '张三',
                  'email': 'zhangsan@example.com',
                  'subject': '应聘后端开发',
                  'body': 'HR 您好，我想投递后端开发岗位，附件是我的简历。',
                  'position': '后端开发',
                  'stage': 'new',
                  'status': 'pending',
                  'has_resume': true,
                  'has_cover_letter': true,
                  'resume_attachments': [
                    {
                      'file_name': '张三-后端开发简历.pdf',
                      'content_type': 'application/pdf',
                    },
                  ],
                  'updated_at': '2026-09-04T00:00:00Z',
                },
              ]),
            ),
            200,
            headers: {'content-type': 'application/json; charset=utf-8'},
          );
        }
        if (request.method == 'POST' &&
            request.url.path ==
                '/api/v1/recruitment/candidates/cand_001/resume/0/view') {
          requestedResumeView = true;
          expect(request.headers['X-Operator'], 'tester');
          return http.Response.bytes(
            utf8.encode(
              jsonEncode({
                'url': '/api/v1/recruitment/resume-view/token_001',
                'expires_at': '2026-09-05T06:00:00Z',
              }),
            ),
            201,
            headers: {'content-type': 'application/json; charset=utf-8'},
          );
        }
        return http.Response('not found', 404);
      }),
    );

    await tester.pumpWidget(
      MaterialApp(home: RecruitmentPage(apiClient: client)),
    );
    await tester.pumpAndSettle();

    expect(find.text('张三-后端开发简历.pdf'), findsOneWidget);
    expect(requestedResumeView, isFalse);
    expect(find.text('预览'), findsOneWidget);
    expect(find.text('下载'), findsOneWidget);

    await tester.tap(find.widgetWithText(OutlinedButton, '预览'));
    await tester.pumpAndSettle();

    expect(requestedResumeView, isTrue);
    expect(find.textContaining('已生成简历预览'), findsOneWidget);
    expect(find.text('简历预览'), findsOneWidget);
    expect(find.text('张三-后端开发简历.pdf'), findsNWidgets(2));
    expect(find.textContaining('PDF 已在下方内嵌预览'), findsOneWidget);
  });

  testWidgets(
    'Recruitment page does not auto-open the first resume attachment',
    (tester) async {
      var resumeViewRequests = 0;
      final client = RecruitmentApiClient(
        baseUrl: 'https://api.example.test',
        operator: 'tester',
        httpClient: MockClient((request) async {
          if (request.method == 'GET' &&
              request.url.path == '/api/v1/recruitment/candidates') {
            return http.Response.bytes(
              utf8.encode(
                jsonEncode([
                  {
                    'id': 'cand_001',
                    'name': '张三',
                    'email': 'zhangsan@example.com',
                    'subject': '应聘后端开发',
                    'body': 'HR 您好，我想投递后端开发岗位，附件是我的简历。',
                    'position': '后端开发',
                    'stage': 'new',
                    'status': 'pending',
                    'has_resume': true,
                    'has_cover_letter': true,
                    'resume_attachments': [
                      {
                        'file_name': '张三-后端开发简历.pdf',
                        'content_type': 'application/pdf',
                      },
                    ],
                    'updated_at': '2026-09-04T00:00:00Z',
                  },
                ]),
              ),
              200,
              headers: {'content-type': 'application/json; charset=utf-8'},
            );
          }
          if (request.method == 'POST' &&
              request.url.path ==
                  '/api/v1/recruitment/candidates/cand_001/resume/0/view') {
            resumeViewRequests++;
            return http.Response.bytes(
              utf8.encode(
                jsonEncode({
                  'url': '/api/v1/recruitment/resume-view/token_001',
                  'expires_at': '2026-09-05T06:00:00Z',
                }),
              ),
              201,
              headers: {'content-type': 'application/json; charset=utf-8'},
            );
          }
          return http.Response('not found', 404);
        }),
      );

      await tester.pumpWidget(
        MaterialApp(home: RecruitmentPage(apiClient: client)),
      );
      await tester.pumpAndSettle();

      expect(resumeViewRequests, 0);
      expect(find.text('简历预览'), findsNothing);
      expect(find.text('预览'), findsOneWidget);
      expect(find.text('下载'), findsOneWidget);
    },
  );

  testWidgets('Recruitment page sorts inbox by latest received time', (
    tester,
  ) async {
    await tester.pumpWidget(
      MaterialApp(
        home: RecruitmentPage(apiClient: clientForCandidates(candidatesJson())),
      ),
    );
    await tester.pumpAndSettle();

    final newTop = tester.getTopLeft(find.widgetWithText(InkWell, '新候选人')).dy;
    final oldTop = tester.getTopLeft(find.widgetWithText(InkWell, '旧候选人')).dy;

    expect(newTop, lessThan(oldTop));
    expect(find.text('新候选人'), findsNWidgets(2));
  });

  testWidgets(
    'Recruitment page keeps latest received mail above updated items',
    (tester) async {
      final candidates = candidatesJson();
      candidates[1]['received_at'] = '2026-09-04T10:00:00Z';
      candidates[1]['updated_at'] = '2026-09-04T10:00:00Z';
      candidates[2]['received_at'] = '2026-09-02T08:00:00Z';
      candidates[2]['updated_at'] = '2026-09-05T12:00:00Z';

      await tester.pumpWidget(
        MaterialApp(
          home: RecruitmentPage(apiClient: clientForCandidates(candidates)),
        ),
      );
      await tester.pumpAndSettle();

      final newTop = tester.getTopLeft(find.widgetWithText(InkWell, '新候选人')).dy;
      final processedTop = tester
          .getTopLeft(find.widgetWithText(InkWell, '已处理候选人'))
          .dy;

      expect(newTop, lessThan(processedTop));
    },
  );

  testWidgets(
    'Recruitment page keeps provider order for equal received times',
    (tester) async {
      final candidates = candidatesJson();
      candidates[0]['received_at'] = '2026-09-04T10:00:00Z';
      candidates[1]['received_at'] = '2026-09-04T10:00:00Z';

      await tester.pumpWidget(
        MaterialApp(
          home: RecruitmentPage(apiClient: clientForCandidates(candidates)),
        ),
      );
      await tester.pumpAndSettle();

      final oldTop = tester.getTopLeft(find.widgetWithText(InkWell, '旧候选人')).dy;
      final newTop = tester.getTopLeft(find.widgetWithText(InkWell, '新候选人')).dy;

      expect(oldTop, lessThan(newTop));
    },
  );

  testWidgets('Recruitment stat cards filter the inbox list', (tester) async {
    await tester.pumpWidget(
      MaterialApp(
        home: RecruitmentPage(apiClient: clientForCandidates(candidatesJson())),
      ),
    );
    await tester.pumpAndSettle();

    expect(find.text('未自动处理'), findsOneWidget);
    expect(find.byKey(const ValueKey('inbox-filter-onlyResume')), findsNothing);

    await tester.tap(find.byKey(const ValueKey('inbox-filter-unprocessed')));
    await tester.pumpAndSettle();
    expect(find.widgetWithText(InkWell, '旧候选人'), findsNothing);
    expect(find.widgetWithText(InkWell, '新候选人'), findsOneWidget);
    expect(find.widgetWithText(InkWell, '已处理候选人'), findsNothing);

    final newTop = tester.getTopLeft(find.widgetWithText(InkWell, '新候选人')).dy;
    expect(newTop.isFinite, isTrue);

    await tester.tap(find.byKey(const ValueKey('inbox-filter-hasBody')));
    await tester.pumpAndSettle();
    expect(find.widgetWithText(InkWell, '新候选人'), findsOneWidget);
    expect(find.widgetWithText(InkWell, '已处理候选人'), findsOneWidget);
    expect(find.widgetWithText(InkWell, '旧候选人'), findsNothing);

    await tester.tap(find.byKey(const ValueKey('inbox-filter-processed')));
    await tester.pumpAndSettle();
    expect(find.widgetWithText(InkWell, '旧候选人'), findsOneWidget);
    expect(find.widgetWithText(InkWell, '已处理候选人'), findsOneWidget);
    expect(find.widgetWithText(InkWell, '新候选人'), findsNothing);

    await tester.tap(find.byKey(const ValueKey('inbox-filter-all')));
    await tester.pumpAndSettle();
    expect(find.widgetWithText(InkWell, '新候选人'), findsOneWidget);
    expect(find.widgetWithText(InkWell, '旧候选人'), findsOneWidget);
    expect(find.widgetWithText(InkWell, '已处理候选人'), findsOneWidget);
  });

  testWidgets('Recruitment stat cards expose clickable cursor feedback', (
    tester,
  ) async {
    await tester.pumpWidget(
      MaterialApp(
        home: RecruitmentPage(apiClient: clientForCandidates(candidatesJson())),
      ),
    );
    await tester.pumpAndSettle();

    final totalCard = tester.widget<InkWell>(
      find.byKey(const ValueKey('inbox-filter-all')),
    );

    expect(totalCard.onTap, isNotNull);
    expect(totalCard.mouseCursor, SystemMouseCursors.click);
  });

  testWidgets('Recruitment page persists manual pass before refreshed load', (
    tester,
  ) async {
    var status = 'pending';
    final client = RecruitmentApiClient(
      baseUrl: 'https://api.example.test',
      operator: 'tester',
      httpClient: MockClient((request) async {
        if (request.method == 'GET' &&
            request.url.path == '/api/v1/recruitment/candidates') {
          return http.Response.bytes(
            utf8.encode(
              jsonEncode([
                {
                  'id': 'cand_001',
                  'name': '张三',
                  'email': 'zhangsan@example.com',
                  'subject': '应聘后端开发',
                  'body': 'HR 您好，我想投递后端开发岗位，附件是我的简历。',
                  'position': '后端开发',
                  'stage': 'new',
                  'status': status,
                  'has_resume': true,
                  'has_cover_letter': true,
                  'updated_at': '2026-09-04T00:00:00Z',
                },
              ]),
            ),
            200,
            headers: {'content-type': 'application/json; charset=utf-8'},
          );
        }
        if (request.method == 'PATCH' &&
            request.url.path == '/api/v1/recruitment/candidates/cand_001') {
          final body = jsonDecode(request.body) as Map<String, dynamic>;
          expect(request.headers['X-Recruitment-Permission'], 'write');
          expect(body, {'status': 'passed'});
          status = body['status'] as String;
          return http.Response.bytes(
            utf8.encode(
              jsonEncode({
                'id': 'cand_001',
                'name': '张三',
                'email': 'zhangsan@example.com',
                'subject': '应聘后端开发',
                'body': 'HR 您好，我想投递后端开发岗位，附件是我的简历。',
                'position': '后端开发',
                'stage': 'new',
                'status': status,
                'has_resume': true,
                'has_cover_letter': true,
                'updated_at': '2026-09-04T00:00:00Z',
              }),
            ),
            200,
            headers: {'content-type': 'application/json; charset=utf-8'},
          );
        }
        return http.Response('not found', 404);
      }),
    );

    await tester.pumpWidget(
      MaterialApp(home: RecruitmentPage(apiClient: client)),
    );
    await tester.pumpAndSettle();

    await tester.ensureVisible(find.widgetWithText(FilledButton, '通过'));
    await tester.tap(find.widgetWithText(FilledButton, '通过'));
    await tester.pumpAndSettle();
    expect(find.text('已通过'), findsWidgets);

    await tester.pumpWidget(
      MaterialApp(home: RecruitmentPage(apiClient: client)),
    );
    await tester.pumpAndSettle();

    expect(find.text('已通过'), findsWidgets);
  });

  testWidgets('Recruitment page exposes report and qtrecurit access actions', (
    tester,
  ) async {
    await tester.pumpWidget(const MaterialApp(home: RecruitmentPage()));
    await tester.pumpAndSettle();

    expect(find.text('拉取新邮件'), findsOneWidget);
    expect(find.text('生成报告'), findsOneWidget);
    expect(find.text('dry_run'), findsOneWidget);
    expect(find.textContaining('未配置 provider API'), findsOneWidget);
  });

  testWidgets('Recruitment page can check provider CLI and mailbox status', (
    tester,
  ) async {
    final client = RecruitmentApiClient(
      baseUrl: 'https://api.example.test',
      operator: 'tester',
      httpClient: MockClient((request) async {
        if (request.method == 'GET' &&
            request.url.path == '/api/v1/recruitment/candidates') {
          return http.Response.bytes(
            utf8.encode(jsonEncode(candidatesJson())),
            200,
            headers: {'content-type': 'application/json; charset=utf-8'},
          );
        }
        if (request.method == 'GET' &&
            request.url.path == '/api/v1/recruitment/provider/status') {
          return http.Response.bytes(
            utf8.encode(
              jsonEncode({
                'status': 'blocked',
                'ready': false,
                'mailbox': 'hr@quanttide.com',
                'message': 'provider 环境未就绪',
                'components': [
                  {
                    'name': 'qtrecurit',
                    'status': 'ok',
                    'message': 'qtrecurit 可执行',
                    'version': 'qtrecurit 0.1.0',
                  },
                  {
                    'name': 'hr_mailbox',
                    'status': 'failed',
                    'message': 'HR 邮箱认证态不可用',
                  },
                ],
                'checked_at': '2026-09-05T06:00:00Z',
              }),
            ),
            200,
            headers: {'content-type': 'application/json; charset=utf-8'},
          );
        }
        return http.Response('not found', 404);
      }),
    );

    await tester.pumpWidget(
      MaterialApp(home: RecruitmentPage(apiClient: client)),
    );
    await tester.pumpAndSettle();

    await tester.tap(find.widgetWithText(OutlinedButton, '检测环境'));
    await tester.pumpAndSettle();

    expect(find.textContaining('provider 环境未就绪'), findsWidgets);
    expect(find.textContaining('HR 邮箱认证态不可用'), findsOneWidget);
    expect(find.textContaining('qtrecurit 0.1.0'), findsOneWidget);
  });
}
