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
  searchable: computed('services.[]', function() {
    return get(this, 'searchables.service')
      .add(this.services)
      .search(this.terms);
  }),
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
        proxies[item.Name.replace('-proxy', '')] = true;
      });

    return proxies;
  }),
});
