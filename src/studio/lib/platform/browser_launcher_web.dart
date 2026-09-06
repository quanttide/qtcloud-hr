import 'dart:js_interop';
import 'dart:js_interop_unsafe';

@JS('window')
external JSObject get _window;

Object? openBrowserWindow() {
  return _window.callMethod(
    'open'.toJS,
    ''.toJS,
    '_blank'.toJS,
  );
}

void openBrowserUrl(String url) {
  _window.callMethod('open'.toJS, url.toJS, '_blank'.toJS, 'noopener'.toJS);
}

void navigateBrowserWindow(Object? window, String url) {
  if (window == null) {
    return;
  }
  (window as JSObject).setProperty('location'.toJS, url.toJS);
}
