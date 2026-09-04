import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:qtcloud_hr_studio/screens/recruitment_screen.dart';

void main() {
  testWidgets('Recruitment page exposes report and qtrecurit access actions', (
    tester,
  ) async {
    await tester.pumpWidget(const MaterialApp(home: RecruitmentPage()));
    await tester.pumpAndSettle();

    expect(find.text('生成报告'), findsOneWidget);
    expect(find.text('dry_run'), findsOneWidget);
    expect(find.text('发送问卷'), findsOneWidget);
    expect(find.text('发送实训邀约'), findsOneWidget);
    expect(find.text('发送笔试'), findsOneWidget);
    expect(find.text('生成面试通知'), findsOneWidget);
    expect(find.textContaining('后端动作：send_survey'), findsOneWidget);
    expect(find.textContaining('后端动作：create_interview_notice'), findsOneWidget);
  });
}
