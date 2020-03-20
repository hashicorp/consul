import Controller from '@ember/controller';
import { computed } from '@ember/object';
import WithEventSource from 'consul-ui/mixins/with-event-source';
import WithSearching from 'consul-ui/mixins/with-searching';
import { get } from '@ember/object';
export default Controller.extend(WithEventSource, WithSearching, {
  queryParams: {
    sortBy: 'sort',
    s: {
      as: 'filter',
    },
  },
  init: function() {
    this.searchParams = {
      node: 's',
    };
    this._super(...arguments);
  },
  searchable: computed('items.[]', function() {
    return get(this, 'searchables.node')
      .add(this.items)
      .search(this.terms);
  }),
});
