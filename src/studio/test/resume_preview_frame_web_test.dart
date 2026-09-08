@TestOn('chrome')
library;

import 'dart:typed_data';

import 'package:flutter/widgets.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:web/web.dart' as web;

import 'package:qtcloud_hr_studio/platform/resume_preview_frame_web.dart';

void main() {
  testWidgets('attaches the PDF blob URL when the iframe is created', (
    tester,
  ) async {
    await tester.pumpWidget(
      Directionality(
        textDirection: TextDirection.ltr,
        child: SizedBox(
          width: 320,
          height: 240,
          child: ResumePreviewFrame(
            bytes: Uint8List.fromList(<int>[37, 80, 68, 70]),
            contentType: 'application/pdf',
            title: 'resume.pdf',
          ),
        ),
      ),
    );
    await tester.pump();

    final iframe =
        web.document.querySelector('iframe[title="resume.pdf"]')
            as web.HTMLIFrameElement?;

    expect(iframe, isNotNull);
    expect(iframe!.src, startsWith('blob:'));
  });
}
