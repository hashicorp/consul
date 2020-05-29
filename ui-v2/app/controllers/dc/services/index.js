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
  actions: {
    healthStatusComparator: function(key, serviceA, serviceB) {
      const [, dir] = key.split(':');
      let a, b;
      if (dir === 'asc') {
        b = serviceA;
        a = serviceB;
      } else {
        a = serviceA;
        b = serviceB;
      }
      switch (true) {
        case a.ChecksCritical > b.ChecksCritical:
          return 1;
        case a.ChecksCritical < b.ChecksCritical:
          return -1;
        default:
          switch (true) {
            case a.ChecksWarning > b.ChecksWarning:
              return 1;
            case a.ChecksWarning < b.ChecksWarning:
              return -1;
            default:
              switch (true) {
                case a.ChecksPassing < b.ChecksPassing:
                  return 1;
                case a.ChecksPassing > b.ChecksPassing:
                  return -1;
              }
          }
          return 0;
      }
    },
  },
});
