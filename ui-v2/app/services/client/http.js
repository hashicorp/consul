import Service, { inject as service } from '@ember/service';
import { get, set } from '@ember/object';
import { Promise } from 'rsvp';

import getObjectPool from 'consul-ui/utils/get-object-pool';
import Request from 'consul-ui/utils/http/request';

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
export default Service.extend({
  dom: service('dom'),
  init: function() {
    this._super(...arguments);
    let protocol = 'http/1.1';
    try {
      protocol = performance.getEntriesByType('resource').reduce(function(prev, item) {
        // for the moment assume script is likely to be
        // the same as xmlhttprequest origins
        if (item.initiatorType === 'script') {
          prev = item.nextHopProtocol;
        }
        return prev;
      }, 'http/1.1');
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
  request: function(options, xhr) {
    const request = new Request(options.type, options.url, { body: options.data || {} }, xhr);
    return get(this, 'connections').acquire(request, request.getId());
  },
  complete: function() {
    return get(this, 'connections').release(...arguments);
  },
});
