import Service from '@ember/service';

import Request from 'consul-ui/utils/http/request';
import createHeaders from 'consul-ui/utils/create-headers';

const parseHeaders = createHeaders();

class HTTPError extends Error {
  constructor(statusCode, message) {
    super(message);
    this.statusCode = statusCode;
  }
}
const xhr = function(options) {
  const xhr = new XMLHttpRequest();
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

export default Service.extend({
  request: function(params) {
    const request = new Request(params.method, params.url, { body: params.data || {} });
    const options = {
      ...params,
      beforeSend: function(xhr) {
        request.open(xhr);
      },
      converters: {
        'text json': function(response) {
          try {
            return JSON.parse(response);
          } catch (e) {
            return response;
          }
        },
      },
      success: function(headers, response, status, statusText) {
        // Response-ish
        request.respond({
          headers: headers,
          response: response,
          status: status,
          statusText: statusText,
        });
      },
      error: function(headers, response, status, statusText, err) {
        let error;
        if (err instanceof Error) {
          error = err;
        } else {
          error = new HTTPError(status, response);
        }
        request.error(error);
      },
      complete: function(status) {
        request.close();
      },
    };
    request.fetch = function() {
      xhr(options);
    };
    return request;
  },
});
