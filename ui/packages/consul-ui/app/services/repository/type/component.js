import LazyProxyService from 'consul-ui/services/lazy-proxy';

import { fromPromise, proxy } from 'consul-ui/utils/dom/event-source';
export default LazyProxyService.extend({
  shouldProxy: function(content, method) {
    return method.indexOf('find') === 0 || method === 'persist';
  },
  execute: function(repo, findOrPersist) {
    return function() {
      return proxy(
        fromPromise(repo[findOrPersist](...arguments)),
        findOrPersist.indexOf('All') === -1 ? {} : []
      );
    };
  },
});
