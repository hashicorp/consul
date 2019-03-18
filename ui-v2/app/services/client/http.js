import Service, { inject as service } from '@ember/service';
import { get, set } from '@ember/object';
import { Promise } from 'rsvp';

import getObjectPool from 'consul-ui/utils/get-object-pool';
import Request from 'consul-ui/utils/http/request';
import createURL from 'consul-ui/utils/createURL';

import { AbortError } from 'ember-data/adapters/errors';

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
const url = createURL(encodeURIComponent);
export default Service.extend({
  dom: service('dom'),
  init: function() {
    this._super(...arguments);
    let protocol = 'http/1.1';
    try {
      protocol = performance.getEntriesByType('resource').find(item => {
        // isCurrent is added in initializers/client and is used
        // to ensure we use the consul-ui.js src to sniff what the protocol
        // is. Based on the assumption that whereever this script is it's
        // likely to be the same as the xmlhttprequests
        return item.initiatorType === 'script' && this.isCurrent(item.name);
      }).nextHopProtocol;
    } catch (e) {
      // pass through
    }
    let maxConnections;
    // http/2, http2+QUIC/39 and SPDY don't have connection limits
    switch (true) {
      case protocol.indexOf('h2') === 0:
      case protocol.indexOf('hq') === 0:
      case protocol.indexOf('spdy') === 0:
        break;
      default:
        // generally 6 are available
        // reserve 1 for traffic that we can't manage
        maxConnections = 5;
        break;
    }
    set(this, 'connections', getObjectPool(dispose, maxConnections));
    if (typeof maxConnections !== 'undefined') {
      set(this, 'maxConnections', maxConnections);
      const doc = get(this, 'dom').document();
      // when the user hides the tab, abort all connections
      doc.addEventListener('visibilitychange', e => {
        if (e.target.hidden) {
          get(this, 'connections').purge();
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
      const doubleBreak = strs.reduce(function(prev, item, i) {
        if (item.indexOf('\n\n') !== -1) {
          return i;
        }
        return prev;
      }, -1);
      let body;
      if (doubleBreak !== -1) {
        // strs.splice(doubleBreak);
        body = values.splice(doubleBreak).reduce(function(prev, item) {
          return {
            ...prev,
            ...item,
          };
        }, {});
      }
      let temp = url(strs, ...values).split(' ');
      const method = temp.shift();
      let rest = temp.join(' ');
      temp = rest.split('\n');
      const path = temp.shift().trim();
      const headers = temp.join('\n').trim();
      return new Promise(function(resolve, reject) {
        const options = {
          url: path,
          method: method,
          complete: function(xhr, textStatus) {
            client.complete(this.id);
          },
          success: function(response, status, xhr) {
            const headers = xhr.getAllResponseHeaders();
            const respond = function(cb) {
              return cb(headers, response);
            };
            resolve(respond);
          },
          error: function(xhr, textStatus, err) {
            reject(err);
          },

          contentType: 'application/json; charset=utf-8',
          converters: {
            'text json': function(response) {
              try {
                return JSON.parse(response);
              } catch (e) {
                return response;
              }
            },
          },
        };
        options.beforeSend = function(xhr) {
          if (headers) {
            Object.keys(headers).forEach(key => xhr.setRequestHeader(key, headers[key]));
          }
          this.id = client.acquire(this, xhr);
        };
        if (body) {
          options.data = body;
        }
        return $.ajax(options).catch(function(e) {
          if (e instanceof AbortError) {
            e.errors[0].status = '0';
          }
          throw e;
        });
      });
    });
  },
  abort: function(id = null) {
    get(this, 'connections').purge();
  },
  whenAvailable: function(e) {
    const doc = get(this, 'dom').document();
    // if we are using a connection limited protocol and the user has hidden the tab (hidden browser/tab switch)
    // any aborted errors should restart
    if (typeof get(this, 'maxConnections') !== 'undefined' && doc.hidden) {
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
    return get(this, 'connections').acquire(request, request.getId());
  },
  complete: function() {
    return get(this, 'connections').release(...arguments);
  },
});
