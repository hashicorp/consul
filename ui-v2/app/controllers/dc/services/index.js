import Controller from '@ember/controller';
import { computed } from '@ember/object';
import WithEventSource from 'consul-ui/mixins/with-event-source';
export default Controller.extend(WithEventSource, {
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
            proxies[service] = true;
          });
        }
      });

    return proxies;
  }),
});
