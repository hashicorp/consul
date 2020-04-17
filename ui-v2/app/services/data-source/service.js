import Service, { inject as service } from '@ember/service';

import MultiMap from 'mnemonist/multi-map';

// TODO: Expose sizes of things via env vars

// caches cursors and previous events when the EventSources are destroyed
let cache;
// keeps a record of currently in use EventSources
let sources;
// keeps a count of currently in use EventSources
let usage;

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
  willDestroy: function() {
    this._listeners.remove();
    Object.entries(sources || {}).forEach(function([key, item]) {
      item.close();
    });
    cache = null;
    sources = null;
    usage = null;
  },

  open: function(uri, ref) {
    let source;
    // Check the cache for an EventSource that is already being used
    // for this uri. If we don't have one, set one up.
    if (uri.indexOf('://') === -1) {
      uri = `consul://${uri}`;
    }
    if (!sources.has(uri)) {
      let [providerName, pathname] = uri.split('://');
      if (pathname.startsWith('//')) {
        pathname = pathname.substr(2);
      }
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
          cache.set(uri, {
            currentEvent: source.getCurrentEvent(),
            cursor: source.configuration.cursor,
          });
          // the data is cached delete the EventSource
          sources.delete(uri);
        },
      });
      sources.set(uri, source);
    } else {
      source = sources.get(uri);
    }
    // set/increase the usage counter
    usage.set(source, ref);
    source.open();
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
