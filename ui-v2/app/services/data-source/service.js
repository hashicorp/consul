import Service, { inject as service } from '@ember/service';
import { proxy } from 'consul-ui/utils/dom/event-source';

import MultiMap from 'mnemonist/multi-map';

// TODO: Expose sizes of things via env vars

// caches cursors and previous events when the EventSources are destroyed
let cache = null;
// keeps a record of currently in use EventSources
let sources = null;
// keeps a count of currently in use EventSources
let usage = null;
export default Service.extend({
  dom: service('dom'),
  encoder: service('encoder'),
  consul: service('data-source/protocols/http'),
  settings: service('data-source/protocols/local-storage'),

  init: function() {
    this._super(...arguments);
    cache = new Map();
    sources = new Map();
    usage = new MultiMap(Set);
    this._listeners = this.dom.listeners();
  },
  resetCache: function() {
    cache = new Map();
  },
  willDestroy: function() {
    this._listeners.remove();
    sources.forEach(function(item) {
      item.close();
    });
    cache = null;
    sources = null;
    usage.clear();
    usage = null;
  },
  source: function(cb, attrs) {
    const src = cb(this.encoder.uriTag());
    return new Promise((resolve, reject) => {
      const ref = {};
      const source = this.open(src, ref, true);
      source.configuration.ref = ref;
      const remove = this._listeners.add(source, {
        message: e => {
          remove();
          // the source only gets wrapped in the proxy
          // after the first message
          // but the proxy itself is resolve to the route
          resolve(proxy(e.target, e.data));
        },
        error: e => {
          remove();
          this.close(source, ref);
          reject(e.error);
        },
      });
      if (typeof source.getCurrentEvent() !== 'undefined') {
        source.dispatchEvent(source.getCurrentEvent());
      }
    });
  },
  unwrap: function(src, ref) {
    const source = src._source;
    usage.set(source, ref);
    usage.remove(source, source.configuration.ref);
    delete source.configuration.ref;
    return source;
  },
  open: function(uri, ref, open = false) {
    if (typeof uri !== 'string') {
      return this.unwrap(uri, ref);
    }
    let source;
    // Check the cache for an EventSource that is already being used
    // for this uri. If we don't have one, set one up.
    if (uri.indexOf('://') === -1) {
      uri = `consul://${uri}`;
    }
    let [providerName, pathname] = uri.split('://');
    const provider = this[providerName];
    if (!sources.has(uri)) {
      let configuration = {};
      if (cache.has(uri)) {
        configuration = cache.get(uri);
      }
      configuration.uri = uri;
      source = provider.source(pathname, configuration);
      const remove = this._listeners.add(source, {
        close: e => {
          // a close could be fired either by:
          // 1. A non-blocking query leaving the page
          // 2. A non-blocking query responding
          // 3. A blocking query responding when is in a closing state
          // 3. A non-blocking query or a blocking query being cancelled
          const source = e.target;
          const event = source.getCurrentEvent();
          const cursor = source.configuration.cursor;
          // only cache data if we have any
          if (typeof event !== 'undefined' && typeof cursor !== 'undefined') {
            cache.set(uri, {
              currentEvent: event,
              cursor: cursor,
            });
          }
          // the data is cached delete the EventSource
          if (!usage.has(source)) {
            // A non-blocking query could close but still be on the page
            sources.delete(uri);
          }
          remove();
        },
      });
      sources.set(uri, source);
    } else {
      source = sources.get(uri);
      // bump to the end of the list
      sources.delete(uri);
      sources.set(uri, source);
    }
    // only open if its not already being used
    // in the case of blocking queries being disabled
    // you may want to specifically force an open
    // if blocking queries are enabled then opening an already
    // open blocking query does nothing
    if (!usage.has(source) || source.readyState > 1 || open) {
      source.open();
    }
    // set/increase the usage counter
    usage.set(source, ref);
    return source;
  },
  close: function(source, ref) {
    // this close is called when the source has either left the page
    // or in the case of a proxied source, it errors
    if (source) {
      // decrease the usage counter
      usage.remove(source, ref);
      // if the EventSource is no longer being used
      // close it (data caching is dealt with by the above 'close' event listener)
      if (!usage.has(source)) {
        source.close();
        if (source.readyState === 2) {
          // in the case that a non-blocking query is on the page
          // and it has already responded and has therefore been cached
          // but not removed itself from sources
          // delete from sources
          sources.delete(source.configuration.uri);
        }
      }
    }
  },
  closed: function() {
    // anything that is closed or closing
    return [...sources.entries()]
      .filter(([key, item]) => {
        return item.readyState > 1;
      })
      .map(item => item[0]);
  },
});
