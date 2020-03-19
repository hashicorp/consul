import Controller from '@ember/controller';
import { get, computed } from '@ember/object';
import WithEventSource from 'consul-ui/mixins/with-event-source';
import WithSearching from 'consul-ui/mixins/with-searching';
export default Controller.extend(WithEventSource, WithSearching, {
  queryParams: {
    sortBy: 'sort',
    s: {
      as: 'filter',
    },
  },
  init: function() {
    this.searchParams = {
      service: 's',
    };
    this._super(...arguments);
  },
  searchable: computed('items.[]', function() {
    return get(this, 'searchables.service')
      .add(this.items)
      .search(this.terms);
  }),
  services: computed('items.[]', function() {
    return this.items.filter(function(item) {
      return item.Kind === 'consul';
    });
  }),
  proxies: computed('items.[]', function() {
    return this.items.filter(function(item) {
      return item.Kind === 'connect-proxy';
    });
  }),
  withProxies: computed('proxies', function() {
    const proxies = {};
    this.proxies.forEach(item => {
      proxies[item.Name.replace('-proxy', '')] = true;
    });
    return proxies;
  }),
  sortOptions: [
    {
      key: 'Name:asc',
      value: 'A to Z',
    },
    {
      key: 'Name:desc',
      value: 'Z to A',
    },
  ],
});
