import 'package:flutter_test/flutter_test.dart';
import 'package:qtcloud_hr_studio/main.dart';

void main() {
  testWidgets('Workbench renders with navigation rail', (
    WidgetTester tester,
  ) async {
    await tester.pumpWidget(
      const WorkbenchApp(initialAccessToken: 'test-access-token'),
    );

    // 侧边导航栏：总览为主页（导航 label + 页面标题各一处）
    expect(find.text('总览'), findsNWidgets(2));
    expect(find.text('计划'), findsOneWidget);
    expect(find.text('招聘'), findsOneWidget);
    expect(find.text('培训'), findsOneWidget);
    expect(find.text('绩效'), findsOneWidget);
    expect(find.text('薪酬'), findsOneWidget);
    expect(find.text('关系'), findsOneWidget);
  });
}
