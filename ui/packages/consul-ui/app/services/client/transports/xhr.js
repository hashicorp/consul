import Service from '@ember/service';

import createHeaders from 'consul-ui/utils/http/create-headers';
import createXHR from 'consul-ui/utils/http/xhr';
import Request from 'consul-ui/utils/http/request';
import HTTPError from 'consul-ui/utils/http/error';

const xhr = createXHR(createHeaders());

export default class XhrService extends Service {
  xhr(options) {
    return xhr(options);
  }

  request(params) {
    const request = new Request(params.method, params.url, {
      ['x-request-id']: params.clientHeaders['x-request-id'],
      body: params.data || {},
    });
    const options = {
      ...params,
      beforeSend: function (xhr) {
        request.open(xhr);
      },
      converters: {
        'text json': function (response) {
          try {
            return JSON.parse(response);
          } catch (e) {
            return response;
          }
        },
      },
      success: function (headers, response, status, statusText) {
        // Response-ish
        request.respond({
          headers: headers,
          response: response,
          status: status,
          statusText: statusText,
        });
      },
      error: function (headers, response, status, statusText, err) {
        let error;
        if (err instanceof Error) {
          error = err;
        } else {
          error = new HTTPError(status, response);
        }
        request.error(error);
      },
      complete: function (status) {
        request.close();
      },
    };
    request.fetch = () => {
      this.xhr(options);
      return request;
    };
    return request;
  }
}
