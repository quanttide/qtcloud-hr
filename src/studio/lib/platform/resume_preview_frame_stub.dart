import 'dart:typed_data';

import 'package:flutter/material.dart';

class ResumePreviewFrame extends StatelessWidget {
  const ResumePreviewFrame({
    super.key,
    required this.bytes,
    required this.contentType,
    required this.title,
  });

  final Uint8List bytes;
  final String contentType;
  final String title;

  @override
  Widget build(BuildContext context) {
    return Container(
      color: const Color(0xFFF8FAFC),
      alignment: Alignment.center,
      child: Text(
        '$title 已加载预览内容。',
        textAlign: TextAlign.center,
        style: TextStyle(fontSize: 13, color: Colors.grey.shade700),
      ),
    );
  }
}
