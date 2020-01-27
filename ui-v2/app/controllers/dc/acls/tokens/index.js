import Controller from '@ember/controller';
import { computed, get } from '@ember/object';
import WithSearching from 'consul-ui/mixins/with-searching';
export default Controller.extend(WithSearching, {
  queryParams: {
    s: {
      as: 'filter',
      replace: true,
    },
  },
  init: function() {
    this.searchParams = {
      token: 's',
    };
    this._super(...arguments);
  },
  searchable: computed('items', function() {
    return get(this, 'searchables.token')
      .add(get(this, 'items'))
      .search(get(this, this.searchParams.token));
  }),
  actions: {
    sendClone: function(item) {
      this.send('clone', item);
    },
  },
});
