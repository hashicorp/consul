/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: MPL-2.0
 */

export default function (parseHeaders, XHR) {
  return function (options) {
    const xhr = new (XHR || XMLHttpRequest)();
    xhr.onreadystatechange = function () {
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
    let url = options.url;
    if (url.endsWith('?')) {
      url = url.substr(0, url.length - 1);
    }
    xhr.open(options.method, url, true);
    if (typeof options.headers === 'undefined') {
      options.headers = {};
    }
    const headers = {
      ...options.headers,
      'X-Requested-With': 'XMLHttpRequest',
    };
    Object.entries(headers).forEach(([key, value]) => xhr.setRequestHeader(key, value));
    options.beforeSend(xhr);
    xhr.withCredentials = true;
    xhr.send(options.body);
    return xhr;
  };
}
