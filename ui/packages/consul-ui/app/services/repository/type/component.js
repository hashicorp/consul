import LazyProxyService from 'consul-ui/services/lazy-proxy';

import { fromPromise, proxy } from 'consul-ui/utils/dom/event-source';
export default class ComponentService extends LazyProxyService {
  shouldProxy(content, method) {
    return method.indexOf('find') === 0 || method === 'persist';
  }

  execute(repo, findOrPersist) {
    return function() {
      return proxy(
        fromPromise(repo[findOrPersist](...arguments)),
        findOrPersist.indexOf('All') === -1 ? {} : []
      );
    };
  }
}
