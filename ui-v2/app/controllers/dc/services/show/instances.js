import Controller from '@ember/controller';
import { get, computed } from '@ember/object';
import { alias } from '@ember/object/computed';
import { inject as service } from '@ember/service';
import WithSearching from 'consul-ui/mixins/with-searching';

export default Controller.extend(WithSearching, {
  dom: service('dom'),
  items: alias('item.Nodes'),
  queryParams: {
    s: {
      as: 'filter',
      replace: true,
    },
  },
  init: function() {
    this.searchParams = {
      serviceInstance: 's',
    };
    this._super(...arguments);
  },
  searchable: computed('items', function() {
    return get(this, 'searchables.serviceInstance')
      .add(this.items)
      .search(get(this, this.searchParams.serviceInstance));
  }),
  keyedProxies: computed('proxies.[]', function() {
    const proxies = {};
    this.proxies.forEach(item => {
      proxies[item.ServiceProxy.DestinationServiceID] = true;
    });

    return proxies;
  }),
});
