import Service, { inject as service } from '@ember/service';
import { get } from '@ember/object';
import { next } from '@ember/runloop';

import { CACHE_CONTROL, CONTENT_TYPE } from 'consul-ui/utils/http/headers';
import { HEADERS_TOKEN as CONSUL_TOKEN } from 'consul-ui/utils/http/consul';

import createURL from 'consul-ui/utils/http/create-url';
import createHeaders from 'consul-ui/utils/http/create-headers';
import createQueryParams from 'consul-ui/utils/http/create-query-params';

// reopen EventSources if a user changes tab
export const restartWhenAvailable = function(client) {
  return function(e) {
    // setup the aborted connection restarting
    // this should happen here to avoid cache deletion
    const status = get(e, 'errors.firstObject.status');
    if (status === '0') {
      // Any '0' errors (abort) should possibly try again, depending upon the circumstances
      // whenAvailable returns a Promise that resolves when the client is available
      // again
      return client.whenAvailable(e);
    }
    throw e;
  };
};
const stringifyQueryParams = createQueryParams(encodeURIComponent);
const parseURL = createURL(encodeURIComponent, stringifyQueryParams);
const parseHeaders = createHeaders();

const parseBody = function(strs, ...values) {
  let body = {};
  const doubleBreak = strs.reduce(function(prev, item, i) {
    // Ensure each line has no whitespace either end, including empty lines
    item = item
      .split('\n')
      .map(item => item.trim())
      .join('\n');
    if (item.indexOf('\n\n') !== -1) {
      return i;
    }
    return prev;
  }, -1);
  if (doubleBreak !== -1) {
    // This merges request bodies together, so you can specify multiple bodies
    // in the request and it will merge them together.
    // Turns out we never actually do this, so it might be worth removing as it complicates
    // matters slightly as we assumed post bodies would be an object.
    // This actually works as it just uses the value of the first object, if its an array
    // it concats
    body = values.splice(doubleBreak).reduce(function(prev, item, i) {
      switch (true) {
        case Array.isArray(item):
          if (i === 0) {
            prev = [];
          }
          return prev.concat(item);
        case typeof item !== 'string':
          return {
            ...prev,
            ...item,
          };
        default:
          return item;
      }
    }, body);
  }
  return [body, ...values];
};

const CLIENT_HEADERS = [CACHE_CONTROL, 'X-Request-ID'];
export default Service.extend({
  dom: service('dom'),
  connections: service('client/connections'),
  transport: service('client/transports/xhr'),
  settings: service('settings'),
  init: function() {
    this._super(...arguments);
    this._listeners = this.dom.listeners();
  },
  willDestroy: function() {
    this._listeners.remove();
    this._super(...arguments);
  },
  url: function() {
    return parseURL(...arguments);
  },
  body: function() {
    return parseBody(...arguments);
  },
  requestParams: function(strs, ...values) {
    // first go to the end and remove/parse the http body
    const [body, ...urlVars] = this.body(...arguments);
    // with whats left get the method off the front
    const [method, ...urlParts] = this.url(strs, ...urlVars).split(' ');
    // with whats left use the rest of the line for the url
    // with whats left after the line, use for the headers
    const [url, ...headerParts] = urlParts.join(' ').split('\n');
    const params = {
      url: url.trim(),
      method: method.trim(),
      headers: {
        [CONTENT_TYPE]: 'application/json; charset=utf-8',
        ...parseHeaders(headerParts),
      },
      body: null,
      data: body,
    };
    // Remove and save things that shouldn't be sent in the request
    params.clientHeaders = CLIENT_HEADERS.reduce(function(prev, item) {
      if (typeof params.headers[item] !== 'undefined') {
        prev[item.toLowerCase()] = params.headers[item];
        delete params.headers[item];
      }
      return prev;
    }, {});
    if (typeof body !== 'undefined') {
      // Only read add HTTP body if we aren't GET
      // Right now we do this to avoid having to put data in the templates
      // for write-like actions
      // potentially we should change things so you _have_ to do that
      // as doing it this way is a little magical
      if (params.method !== 'GET') {
        if (params.headers[CONTENT_TYPE].indexOf('json') !== -1) {
          params.body = JSON.stringify(params.data);
        } else {
          if (
            (typeof params.data === 'string' && params.data.length > 0) ||
            Object.keys(params.data).length > 0
          ) {
            params.body = params.data;
          }
        }
      } else {
        const str = stringifyQueryParams(params.data);
        if (str.length > 0) {
          if (params.url.indexOf('?') !== -1) {
            params.url = `${params.url}&${str}`;
          } else {
            params.url = `${params.url}?${str}`;
          }
        }
      }
    }
    // temporarily reset the headers/content-type so it works the same
    // as previously, should be able to remove this once the data layer
    // rewrite is over and we can assert sending via form-encoded is fine
    // also see adapters/kv content-types in requestForCreate/UpdateRecord
    // also see https://github.com/hashicorp/consul/issues/3804
    params.headers[CONTENT_TYPE] = 'application/json; charset=utf-8';
    return params;
  },
  request: function(cb) {
    const client = this;
    return cb(function(strs, ...values) {
      const params = client.requestParams(...arguments);
      return client.settings.findBySlug('token').then(token => {
        const options = {
          ...params,
          headers: {
            [CONSUL_TOKEN]: typeof token.SecretID === 'undefined' ? '' : token.SecretID,
            ...params.headers,
          },
        };
        const request = client.transport.request(options);
        return new Promise((resolve, reject) => {
          const remove = client._listeners.add(request, {
            open: e => {
              client.acquire(e.target);
            },
            message: e => {
              const headers = {
                ...Object.entries(e.data.headers).reduce(function(prev, [key, value], i) {
                  if (!CLIENT_HEADERS.includes(key)) {
                    prev[key] = value;
                  }
                  return prev;
                }, {}),
                ...params.clientHeaders,
              };
              const respond = function(cb) {
                return cb(headers, e.data.response);
              };
              next(() => resolve(respond));
            },
            error: e => {
              next(() => reject(e.error));
            },
            close: e => {
              client.release(e.target);
              remove();
            },
          });
          request.fetch();
        });
      });
    });
  },
  whenAvailable: function(e) {
    return this.connections.whenAvailable(e);
  },
  abort: function() {
    return this.connections.purge(...arguments);
  },
  acquire: function() {
    return this.connections.acquire(...arguments);
  },
  release: function() {
    return this.connections.release(...arguments);
  },
});
