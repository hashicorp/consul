import Service, { inject as service } from '@ember/service';

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
    usage = null;
  },

  open: function(uri, ref, open = false) {
    let source;
    // Check the cache for an EventSource that is already being used
    // for this uri. If we don't have one, set one up.
    if (uri.indexOf('://') === -1) {
      uri = `consul://${uri}`;
    }
    if (!sources.has(uri)) {
      let [providerName, pathname] = uri.split('://');
      const provider = this[providerName];

      let configuration = {};
      if (cache.has(uri)) {
        configuration = cache.get(uri);
      }
      source = provider.source(pathname, configuration);
      this._listeners.add(source, {
        close: e => {
          const source = e.target;
          source.removeEventListener('close', close);
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
            sources.delete(uri);
          }
        },
      });
      sources.set(uri, source);
    } else {
      source = sources.get(uri);
    }
    // only open if its not already being used
    // in the case of blocking queries being disabled
    // you may want to specifically force an open
    // if blocking queries are enabled then opening an already
    // open blocking query does nothing
    if (!usage.has(source) || open) {
      source.open();
    }
    // set/increase the usage counter
    usage.set(source, ref);
    return source;
  },
  close: function(source, ref) {
    if (source) {
      // decrease the usage counter
      usage.remove(source, ref);
      // if the EventSource is no longer being used
      // close it (data caching is dealt with by the above 'close' event listener)
      if (!usage.has(source)) {
        source.close();
      }
    }
  },
});
