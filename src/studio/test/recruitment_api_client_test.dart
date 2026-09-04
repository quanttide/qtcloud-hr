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
            'position': '数据工程师',
            'stage': 'new',
            'status': 'pending',
            'has_resume': true,
            'has_cover_letter': true,
            'last_action': 'send_survey',
            'updated_at': '2026-09-04T00:00:00Z',
          },
        ], 200);
      }),
    );

    final candidates = await client.listCandidates();

    expect(candidates, hasLength(1));
    expect(candidates.single.id, 'cand_001');
    expect(candidates.single.email, 'zhangsan@example.com');
    expect(candidates.single.lastAction, 'send_survey');
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
}

http.Response jsonResponse(Object body, int statusCode) {
  return http.Response.bytes(
    utf8.encode(jsonEncode(body)),
    statusCode,
    headers: {'content-type': 'application/json; charset=utf-8'},
  );
}
