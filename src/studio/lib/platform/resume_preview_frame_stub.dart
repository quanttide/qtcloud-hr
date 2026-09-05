import 'package:flutter/material.dart';

class ResumePreviewFrame extends StatelessWidget {
  const ResumePreviewFrame({
    super.key,
    required this.url,
    required this.title,
  });

  final String url;
  final String title;

  @override
  Widget build(BuildContext context) {
    return Container(
      color: const Color(0xFFF8FAFC),
      alignment: Alignment.center,
      child: Text(
        '$title 已生成临时预览链接，请在浏览器中打开。',
        textAlign: TextAlign.center,
        style: TextStyle(fontSize: 13, color: Colors.grey.shade700),
      ),
    );
  }
}
