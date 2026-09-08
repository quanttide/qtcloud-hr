import 'dart:js_interop';
import 'dart:typed_data';
import 'dart:ui_web' as ui_web;

import 'package:flutter/widgets.dart';
import 'package:web/web.dart' as web;

class ResumePreviewFrame extends StatefulWidget {
  ResumePreviewFrame({
    super.key,
    required this.bytes,
    required this.contentType,
    required this.title,
  }) : _viewType = 'resume-preview-${_nextViewId++}' {
    ui_web.platformViewRegistry.registerViewFactory(_viewType, (int viewId) {
      return web.HTMLIFrameElement()
        ..id = _viewType
        ..title = title
        ..style.border = '0'
        ..style.width = '100%'
        ..style.height = '100%';
    });
  }

  final Uint8List bytes;
  final String contentType;
  final String title;
  final String _viewType;
  static int _nextViewId = 0;

  @override
  State<ResumePreviewFrame> createState() => _ResumePreviewFrameState();
}

class _ResumePreviewFrameState extends State<ResumePreviewFrame> {
  String? _objectUrl;
  int? _viewId;

  @override
  void initState() {
    super.initState();
    _objectUrl = _createObjectUrl(widget.bytes, widget.contentType);
  }

  @override
  void didUpdateWidget(covariant ResumePreviewFrame oldWidget) {
    super.didUpdateWidget(oldWidget);
    if (oldWidget.bytes != widget.bytes ||
        oldWidget.contentType != widget.contentType) {
      _revokeObjectUrl();
      _objectUrl = _createObjectUrl(widget.bytes, widget.contentType);
      _setFrameSource();
    }
  }

  @override
  void dispose() {
    _revokeObjectUrl();
    super.dispose();
  }

  String _createObjectUrl(Uint8List bytes, String contentType) {
    final blob = web.Blob(
      <JSAny>[bytes.toJS].toJS,
      web.BlobPropertyBag(type: contentType),
    );
    return web.URL.createObjectURL(blob);
  }

  void _revokeObjectUrl() {
    final objectUrl = _objectUrl;
    if (objectUrl != null) {
      web.URL.revokeObjectURL(objectUrl);
      _objectUrl = null;
    }
  }

  void _setFrameSource() {
    final viewId = _viewId;
    final objectUrl = _objectUrl;
    if (viewId == null || objectUrl == null) {
      return;
    }
    final element = ui_web.platformViewRegistry.getViewById(viewId);
    if (element.isA<web.HTMLIFrameElement>()) {
      (element as web.HTMLIFrameElement).src = objectUrl;
    }
  }

  @override
  Widget build(BuildContext context) {
    return HtmlElementView(
      viewType: widget._viewType,
      onPlatformViewCreated: (viewId) {
        _viewId = viewId;
        _setFrameSource();
      },
    );
  }
}
