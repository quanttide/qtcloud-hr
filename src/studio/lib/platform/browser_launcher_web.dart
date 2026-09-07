import 'package:web/web.dart' as web;

void downloadBrowserFile(String url, String fileName) {
  final anchor = web.HTMLAnchorElement()
    ..href = url
    ..download = fileName
    ..style.display = 'none';
  web.document.body?.append(anchor);
  anchor.click();
  anchor.remove();
}
