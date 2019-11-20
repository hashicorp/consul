/*global $*/
import Service, { inject as service } from '@ember/service';
import { get, set } from '@ember/object';
import { Promise } from 'rsvp';

import { env } from 'consul-ui/env';
import getObjectPool from 'consul-ui/utils/get-object-pool';
import Request from 'consul-ui/utils/http/request';
import createURL from 'consul-ui/utils/createURL';

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
class HTTPError extends Error {
  constructor(statusCode, message) {
    super(message);
    this.statusCode = statusCode;
  }
}
const dispose = function(request) {
  if (request.headers()['content-type'] === 'text/event-stream') {
    const xhr = request.connection();
    // unsent and opened get aborted
    // headers and loading means wait for it
    // to finish for the moment
    if (xhr.readyState) {
      switch (xhr.readyState) {
        case 0:
        case 1:
          xhr.abort();
          break;
      }
    }
  }
  return request;
};
// TODO: Potentially url should check if any of the params
// passed to it are undefined (null is fine). We could then get rid of the
// multitude of checks we do throughout the adapters
// right now createURL converts undefined to '' so we need to check thats not needed
// anywhere (todo written here for visibility)
const url = createURL(encodeURIComponent);
export default Service.extend({
  dom: service('dom'),
  settings: service('settings'),
  init: function() {
    this._super(...arguments);
    const maxConnections = env('CONSUL_HTTP_MAX_CONNECTIONS');
    set(this, 'connections', getObjectPool(dispose, maxConnections));
    if (typeof maxConnections !== 'undefined') {
      set(this, 'maxConnections', maxConnections);
      const doc = this.dom.document();
      // when the user hides the tab, abort all connections
      doc.addEventListener('visibilitychange', e => {
        if (e.target.hidden) {
          this.connections.purge();
        }
      });
    }
  },
  url: function() {
    return url(...arguments);
  },
  request: function(cb) {
    const client = this;
    return cb(function(strs, ...values) {
      let body = {};
      const doubleBreak = strs.reduce(function(prev, item, i) {
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
      let temp = url(strs, ...values).split(' ');
      const method = temp.shift();
      let rest = temp.join(' ');
      temp = rest.split('\n');
      const path = temp.shift().trim();
      const createHeaders = function(lines) {
        return lines.reduce(function(prev, item) {
          const temp = item.split(':');
          if (temp.length > 1) {
            prev[temp[0].trim()] = temp[1].trim();
          }
          return prev;
        }, {});
      };
      const headers = {
        ...{
          'Content-Type': 'application/json; charset=utf-8',
        },
        ...get(client, 'settings').findHeaders(),
        ...createHeaders(temp),
      };
      return new Promise(function(resolve, reject) {
        const options = {
          url: path,
          method: method,
          contentType: headers['Content-Type'],
          // type: 'json',
          complete: function(xhr, textStatus) {
            client.complete(this.id);
          },
          success: function(response, status, xhr) {
            const headers = createHeaders(xhr.getAllResponseHeaders().split('\n'));
            const respond = function(cb) {
              return cb(headers, response);
            };
            //TODO: nextTick ?
            resolve(respond);
          },
          error: function(xhr, textStatus, err) {
            let error;
            if (err instanceof Error) {
              error = err;
            } else {
              let status = xhr.status;
              // TODO: Not sure if we actually need this, but ember-data checks it
              if (textStatus === 'abort') {
                status = 0;
              }
              if (textStatus === 'timeout') {
                status = 408;
              }
              error = new HTTPError(status, xhr.responseText);
            }
            //TODO: nextTick ?
            reject(error);
          },
          converters: {
            'text json': function(response) {
              try {
                return $.parseJSON(response);
              } catch (e) {
                return response;
              }
            },
          },
        };
        if (typeof body !== 'undefined') {
          // Only read add HTTP body if we aren't GET
          // Right now we do this to avoid having to put data in the templates
          // for write-like actions
          // potentially we should change things so you _have_ to do that
          // as doing it this way is a little magical
          if (method !== 'GET' && headers['Content-Type'].indexOf('json') !== -1) {
            options.data = JSON.stringify(body);
          } else {
            // TODO: Does this need urlencoding? Assuming jQuery does this
            options.data = body;
          }
        }
        // temporarily reset the headers/content-type so it works the same
        // as previously, should be able to remove this once the data layer
        // rewrite is over and we can assert sending via form-encoded is fine
        // also see adapters/kv content-types in requestForCreate/UpdateRecord
        // also see https://github.com/hashicorp/consul/issues/3804
        options.contentType = 'application/json; charset=utf-8';
        headers['Content-Type'] = options.contentType;
        //
        options.beforeSend = function(xhr) {
          if (headers) {
            Object.keys(headers).forEach(key => xhr.setRequestHeader(key, headers[key]));
          }
          this.id = client.acquire(options, xhr);
        };
        return $.ajax(options);
      });
    });
  },
  abort: function(id = null) {
    this.connections.purge();
  },
  whenAvailable: function(e) {
    const doc = this.dom.document();
    // if we are using a connection limited protocol and the user has hidden the tab (hidden browser/tab switch)
    // any aborted errors should restart
    if (typeof this.maxConnections !== 'undefined' && doc.hidden) {
      return new Promise(function(resolve) {
        doc.addEventListener('visibilitychange', function listen(event) {
          doc.removeEventListener('visibilitychange', listen);
          resolve(e);
        });
      });
    }
    return Promise.resolve(e);
  },
  acquire: function(options, xhr) {
    const request = new Request(options.type, options.url, { body: options.data || {} }, xhr);
    return this.connections.acquire(request, request.getId());
  },
  complete: function() {
    return this.connections.release(...arguments);
  },
});
