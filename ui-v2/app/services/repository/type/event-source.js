import { inject as service } from '@ember/service';
import { get } from '@ember/object';

import LazyProxyService from 'consul-ui/services/lazy-proxy';

import { cache as createCache, BlockingEventSource } from 'consul-ui/utils/dom/event-source';

const createProxy = function(repo, find, settings, cache, serialize = JSON.stringify) {
  const client = this.client;
  const store = this.store;
  // custom createEvent, here used to reconcile the ember-data store for each tick
  const createEvent = function(result, configuration) {
    const event = {
      type: 'message',
      data: result,
    };
    const meta = get(event.data || {}, 'meta') || {};
    if (typeof meta.date !== 'undefined') {
      // unload anything older than our current sync date/time
      const checkNspace = meta.nspace !== '';
      store.peekAll(repo.getModelName()).forEach(function(item) {
        const dc = get(item, 'Datacenter');
        if (dc === meta.dc) {
          if (checkNspace) {
            const nspace = get(item, 'Namespace');
            if (nspace !== meta.namespace) {
              return;
            }
          }
          const date = get(item, 'SyncTime');
          if (typeof date !== 'undefined' && date != meta.date) {
            store.unloadRecord(item);
          }
        }
      });
    }
    return event;
  };
  // proxied find*..(id, dc)
  return function() {
    const key = `${repo.getModelName()}.${find}.${serialize([...arguments])}`;
    const _args = arguments;
    const newPromisedEventSource = cache;
    return newPromisedEventSource(
      function(configuration) {
        // take a copy of the original arguments
        let args = [..._args];
        if (configuration.settings.enabled) {
          // ...and only add our current cursor/configuration if we are blocking
          args = args.concat([configuration]);
        }
        // original find... with configuration now added
        return repo[find](...args)
          .then(res => {
            if (!configuration.settings.enabled) {
              // blocking isn't enabled, immediately close
              this.close();
            }
            return res;
          })
          .catch(function(e) {
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
          });
      },
      {
        key: key,
        type: BlockingEventSource,
        settings: {
          enabled: typeof settings.blocking === 'undefined' || settings.blocking,
        },
        createEvent: createEvent,
      }
    );
  };
};
let cache = null;
let cacheMap = null;
export default LazyProxyService.extend({
  store: service('store'),
  settings: service('settings'),
  wait: service('timeout'),
  client: service('client/http'),
  init: function() {
    this._super(...arguments);
    if (cache === null) {
      this.resetCache();
    }
  },
  resetCache: function() {
    Object.entries(cacheMap || {}).forEach(function([key, item]) {
      item.close();
    });
    cacheMap = {};
    cache = createCache(cacheMap);
  },
  willDestroy: function() {
    Object.entries(cacheMap || {}).forEach(function([key, item]) {
      item.close();
    });
    cacheMap = null;
    cache = null;
  },
  shouldProxy: function(content, method) {
    return method.indexOf('find') === 0;
  },
  execute: function(repo, find) {
    return this.settings.findBySlug('client').then(settings => {
      return createProxy.bind(this)(repo, find, settings, cache);
    });
  },
});
