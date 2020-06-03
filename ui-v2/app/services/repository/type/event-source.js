import { inject as service } from '@ember/service';
import { get } from '@ember/object';

import { restartWhenAvailable } from 'consul-ui/services/client/http';
import LazyProxyService from 'consul-ui/services/lazy-proxy';

import { cache as createCache, BlockingEventSource } from 'consul-ui/utils/dom/event-source';

const createProxy = function(repo, find, settings, cache, serialize = JSON.stringify) {
  const client = this.client;
  // custom createEvent, here used to reconcile the ember-data store for each tick
  let createEvent;
  if (repo.shouldReconcile(find)) {
    createEvent = function(result = {}, configuration) {
      const event = {
        type: 'message',
        data: result,
      };
      repo.reconcile(get(event, 'data.meta'));
      return event;
    };
  }
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
          .catch(restartWhenAvailable(client));
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
  settings: service('settings'),
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
