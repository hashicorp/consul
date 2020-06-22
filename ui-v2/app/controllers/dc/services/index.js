import Controller from '@ember/controller';
import { computed } from '@ember/object';

export default Controller.extend({
  queryParams: {
    sortBy: 'sort',
    search: {
      as: 'filter',
    },
  },
  services: computed('items.[]', function() {
    return this.items.filter(function(item) {
      return item.Kind !== 'connect-proxy';
    });
  }),
  proxies: computed('items.[]', function() {
    const proxies = {};
    this.items
      .filter(function(item) {
        return item.Kind === 'connect-proxy';
      })
      .forEach(item => {
        // Iterating to cover the usecase of a proxy being
        // used by more than one service
        if (item.ProxyFor) {
          item.ProxyFor.forEach(service => {
            proxies[service] = item;
          });
        }
      });

    return proxies;
  }),
});
