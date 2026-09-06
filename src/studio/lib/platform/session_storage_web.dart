import 'package:web/web.dart' as web;

String? readSessionValue(String key) {
  return web.window.sessionStorage.getItem(key);
}

void writeSessionValue(String key, String value) {
  web.window.sessionStorage.setItem(key, value);
}

void removeSessionValue(String key) {
  web.window.sessionStorage.removeItem(key);
}
