import Controller from '@ember/controller';
import { get, computed } from '@ember/object';
import WithSearching from 'consul-ui/mixins/with-searching';
export default Controller.extend(WithSearching, {
  queryParams: {
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
});
