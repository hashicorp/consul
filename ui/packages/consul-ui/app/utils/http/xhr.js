export default function(parseHeaders, XHR) {
  return function(options) {
    const xhr = new (XHR || XMLHttpRequest)();
    xhr.onreadystatechange = function() {
      if (this.readyState === 4) {
        const headers = parseHeaders(this.getAllResponseHeaders().split('\n'));
        if (this.status >= 200 && this.status < 400) {
          const response = options.converters['text json'](this.response);
          options.success(headers, response, this.status, this.statusText);
        } else {
          options.error(headers, this.responseText, this.status, this.statusText, this.error);
        }
        options.complete(this.status);
      }
    };
    xhr.open(options.method, options.url, true);
    if (typeof options.headers === 'undefined') {
      options.headers = {};
    }
    const headers = {
      ...options.headers,
      'X-Requested-With': 'XMLHttpRequest',
    };
    Object.entries(headers).forEach(([key, value]) => xhr.setRequestHeader(key, value));
    options.beforeSend(xhr);
    xhr.send(options.body);
    return xhr;
  };
}
