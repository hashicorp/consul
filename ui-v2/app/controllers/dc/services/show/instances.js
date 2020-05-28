import Controller from '@ember/controller';
import { computed } from '@ember/object';
import { alias } from '@ember/object/computed';

export default Controller.extend({
  items: alias('item.Nodes'),
  queryParams: {
    search: {
      as: 'filter',
      replace: true,
    },
  },
  keyedProxies: computed('proxies.[]', function() {
    const proxies = {};
    this.proxies.forEach(item => {
      proxies[item.ServiceProxy.DestinationServiceID] = true;
    });

    return proxies;
  }),
});
