import 'dart:convert';

import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:qtcloud_hr_studio/services/recruitment_api_client.dart';

void main() {
  test('listCandidates maps provider fields into UI candidates', () async {
    final client = RecruitmentApiClient(
      baseUrl: 'https://api.example.test',
      operator: 'tester',
      httpClient: MockClient((request) async {
        expect(request.url.path, '/api/v1/recruitment/candidates');
        expect(request.headers['X-Operator'], 'tester');
        return jsonResponse([
          {
            'id': 'cand_001',
            'name': '张三',
            'email': 'zhangsan@example.com',
            'subject': '应聘后端开发',
            'body': 'HR 您好，我想投递后端开发岗位，附件是我的简历。',
            'position': '数据工程师',
            'stage': 'new',
            'status': 'pending',
            'has_resume': true,
            'has_cover_letter': true,
            'resume_attachments': [
              {
                'file_name': '张三-后端开发简历.pdf',
                'content_type': 'application/pdf',
                'url': 'https://files.example.test/resumes/cand_001.pdf',
                'size': 245760,
              },
            ],
            'last_action': 'send_survey',
            'received_at': '2026-09-03T08:00:00Z',
            'updated_at': '2026-09-04T00:00:00Z',
          },
        ], 200);
      }),
    );

    final candidates = await client.listCandidates();

    expect(candidates, hasLength(1));
    expect(candidates.single.id, 'cand_001');
    expect(candidates.single.email, 'zhangsan@example.com');
    expect(candidates.single.subject, '应聘后端开发');
    expect(candidates.single.body, contains('投递后端开发岗位'));
    expect(candidates.single.resumeAttachments, hasLength(1));
    expect(
      candidates.single.resumeAttachments.single.fileName,
      '张三-后端开发简历.pdf',
    );
    expect(
      candidates.single.resumeAttachments.single.url,
      'https://files.example.test/resumes/cand_001.pdf',
    );
    expect(candidates.single.lastAction, 'send_survey');
    expect(candidates.single.receivedAt, DateTime.utc(2026, 9, 3, 8));
    expect(candidates.single.updatedAt, DateTime.utc(2026, 9, 4));
  });

  test(
    'listCandidates falls back to updated_at when received_at is missing',
    () async {
      final client = RecruitmentApiClient(
        baseUrl: 'https://api.example.test',
        operator: 'tester',
        httpClient: MockClient((request) async {
          return jsonResponse([
            {
              'id': 'cand_001',
              'name': '张三',
              'email': 'zhangsan@example.com',
              'stage': 'new',
              'status': 'pending',
              'has_resume': true,
              'has_cover_letter': true,
              'updated_at': '2026-09-04T00:00:00Z',
            },
          ], 200);
        }),
      );

      final candidates = await client.listCandidates();

      expect(candidates.single.receivedAt, DateTime.utc(2026, 9, 4));
    },
  );

  test('listCandidates parses qtrecurit minute timestamps', () async {
    final client = RecruitmentApiClient(
      baseUrl: 'https://api.example.test',
      operator: 'tester',
      httpClient: MockClient((request) async {
        return jsonResponse([
          {
            'id': 'cand_001',
            'name': '张三',
            'email': 'zhangsan@example.com',
            'stage': 'new',
            'status': 'pending',
            'has_resume': true,
            'has_cover_letter': true,
            'updated_at': '2026-09-04 16:26',
          },
        ], 200);
      }),
    );

    final candidates = await client.listCandidates();

    expect(candidates.single.receivedAt, DateTime(2026, 9, 4, 16, 26));
  });

  test('createReport posts dry_run and returns markdown', () async {
    final client = RecruitmentApiClient(
      baseUrl: 'https://api.example.test',
      operator: 'tester',
      httpClient: MockClient((request) async {
        final body = jsonDecode(request.body) as Map<String, dynamic>;
        expect(request.url.path, '/api/v1/recruitment/reports');
        expect(body['days'], 30);
        expect(body['dry_run'], true);
        return jsonResponse({
          'report_id': 'rpt_001',
          'status': 'dry_run',
          'markdown': '# 招聘统计报告',
          'created_at': '2026-09-04T00:00:00Z',
        }, 201);
      }),
    );

    final report = await client.createReport(days: 30, dryRun: true);

    expect(report.status, 'dry_run');
    expect(report.markdown, contains('招聘统计报告'));
  });

  test(
    'runAction posts whitelisted action payload and permission header',
    () async {
      final client = RecruitmentApiClient(
        baseUrl: 'https://api.example.test',
        operator: 'tester',
        httpClient: MockClient((request) async {
          final body = jsonDecode(request.body) as Map<String, dynamic>;
          expect(
            request.url.path,
            '/api/v1/recruitment/candidates/cand_001/actions',
          );
          expect(request.headers['X-Recruitment-Permission'], 'write');
          expect(body['action'], 'create_interview_notice');
          expect(body['dry_run'], true);
          expect(body['params'], {
            'position': '数据工程师',
            'time': '2026-09-10 10:00',
          });
          return jsonResponse({
            'action_id': 'act_001',
            'candidate_id': 'cand_001',
            'action': 'create_interview_notice',
            'status': 'dry_run',
            'message': '已完成 dry_run 预览，未发送邮件',
            'created_at': '2026-09-04T00:00:00Z',
          }, 201);
        }),
      );

      final result = await client.runAction(
        candidateId: 'cand_001',
        action: RecruitmentAction.createInterviewNotice,
        dryRun: true,
        params: const {'position': '数据工程师', 'time': '2026-09-10 10:00'},
      );

      expect(result.action, 'create_interview_notice');
      expect(result.status, 'dry_run');
    },
  );

  test(
    'updateCandidateStatus patches manual decision with permission header',
    () async {
      final client = RecruitmentApiClient(
        baseUrl: 'https://api.example.test',
        operator: 'tester',
        httpClient: MockClient((request) async {
          final body = jsonDecode(request.body) as Map<String, dynamic>;
          expect(request.method, 'PATCH');
          expect(request.url.path, '/api/v1/recruitment/candidates/cand_001');
          expect(request.headers['X-Operator'], 'tester');
          expect(request.headers['X-Recruitment-Permission'], 'write');
          expect(body, {'status': 'passed'});
          return jsonResponse({
            'id': 'cand_001',
            'name': '张三',
            'email': 'zhangsan@example.com',
            'stage': 'new',
            'status': 'passed',
            'has_resume': true,
            'has_cover_letter': true,
            'updated_at': '2026-09-04T00:00:00Z',
          }, 200);
        }),
      );

      final candidate = await client.updateCandidateStatus(
        candidateId: 'cand_001',
        status: 'passed',
      );

      expect(candidate.status, 'passed');
    },
  );

  test('syncInbox posts controlled mailbox sync payload', () async {
    final client = RecruitmentApiClient(
      baseUrl: 'https://api.example.test',
      operator: 'tester',
      httpClient: MockClient((request) async {
        final body = jsonDecode(request.body) as Map<String, dynamic>;
        expect(request.url.path, '/api/v1/recruitment/inbox/sync');
        expect(request.headers['X-Operator'], 'tester');
        expect(request.headers['X-Recruitment-Permission'], 'write');
        expect(body, {
          'mailbox': 'hr@quanttide.com',
          'folder': 'INBOX',
          'page_size': 50,
          'dry_run': true,
        });
        return jsonResponse({
          'sync_id': 'sync_001',
          'status': 'dry_run',
          'mailbox': 'hr@quanttide.com',
          'folder': 'INBOX',
          'scanned': 0,
          'imported': 0,
          'candidates': [],
          'created_at': '2026-09-04T00:00:00Z',
        }, 200);
      }),
    );

    final result = await client.syncInbox(dryRun: true);

    expect(result.status, 'dry_run');
    expect(result.mailbox, 'hr@quanttide.com');
    expect(result.candidates, isEmpty);
  });

  test('createResumeView requests a short-lived provider view URL', () async {
    final client = RecruitmentApiClient(
      baseUrl: 'https://api.example.test',
      operator: 'tester',
      httpClient: MockClient((request) async {
        expect(request.method, 'POST');
        expect(
          request.url.path,
          '/api/v1/recruitment/candidates/cand_001/resume/0/view',
        );
        expect(request.headers['X-Operator'], 'tester');
        return jsonResponse({
          'url': '/api/v1/recruitment/resume-view/token_001',
          'expires_at': '2026-09-05T06:00:00Z',
        }, 201);
      }),
    );

    final view = await client.createResumeView(
      candidateId: 'cand_001',
      attachmentIndex: 0,
    );

    expect(
      view.url,
      'https://api.example.test/api/v1/recruitment/resume-view/token_001',
    );
    expect(view.expiresAt, DateTime.utc(2026, 9, 5, 6));
  });

  test('errors expose only provider safe message', () async {
    final client = RecruitmentApiClient(
      baseUrl: 'https://api.example.test',
      operator: 'tester',
      httpClient: MockClient((request) async {
        return jsonResponse({'error': '招聘服务暂不可用，请稍后重试'}, 502);
      }),
    );

    await expectLater(
      client.createReport(days: 30, dryRun: false),
      throwsA(
        isA<RecruitmentApiException>().having(
          (error) => error.message,
          'message',
          '招聘服务暂不可用，请稍后重试',
        ),
      ),
    );
  });

  test('non json responses are converted to safe errors', () async {
    final client = RecruitmentApiClient(
      baseUrl: 'https://api.example.test',
      operator: 'tester',
      httpClient: MockClient((request) async {
        return http.Response('<html>not the provider</html>', 404);
      }),
    );

    await expectLater(
      client.listCandidates(),
      throwsA(
        isA<RecruitmentApiException>().having(
          (error) => error.message,
          'message',
          '招聘服务暂不可用，请稍后重试',
        ),
      ),
    );
  });
}

http.Response jsonResponse(Object body, int statusCode) {
  return http.Response.bytes(
    utf8.encode(jsonEncode(body)),
    statusCode,
    headers: {'content-type': 'application/json; charset=utf-8'},
  );
}
