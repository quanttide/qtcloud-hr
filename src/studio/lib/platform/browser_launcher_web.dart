import 'dart:js_interop';
import 'dart:js_interop_unsafe';

@JS('window')
external JSObject get _window;

void openBrowserUrl(String url) {
  _window.callMethod('open'.toJS, url.toJS, '_blank'.toJS, 'noopener'.toJS);
}
