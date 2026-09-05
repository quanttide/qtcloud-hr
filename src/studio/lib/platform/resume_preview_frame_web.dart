import 'dart:ui_web' as ui_web;

import 'package:flutter/widgets.dart';
import 'package:web/web.dart' as web;

class ResumePreviewFrame extends StatelessWidget {
  ResumePreviewFrame({
    super.key,
    required this.url,
    required this.title,
  }) : _viewType = 'resume-preview-${_nextViewId++}' {
    ui_web.platformViewRegistry.registerViewFactory(_viewType, (int viewId) {
      return web.HTMLIFrameElement()
        ..src = url
        ..title = title
        ..style.border = '0'
        ..style.width = '100%'
        ..style.height = '100%';
    });
  }

  final String url;
  final String title;
  final String _viewType;
  static int _nextViewId = 0;

  @override
  Widget build(BuildContext context) {
    return HtmlElementView(viewType: _viewType);
  }
}
