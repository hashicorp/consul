import Controller from '@ember/controller';
import { inject as service } from '@ember/service';
import { get, computed } from '@ember/object';
import { alias } from '@ember/object/computed';
import WithSearching from 'consul-ui/mixins/with-searching';

export default Controller.extend(WithSearching, {
  dom: service('dom'),
  items: alias('item.Services'),
  queryParams: {
    s: {
      as: 'filter',
      replace: true,
    },
  },
  init: function() {
    this.searchParams = {
      nodeservice: 's',
    };
    this._super(...arguments);
  },
  searchable: computed('items', function() {
    return get(this, 'searchables.nodeservice')
      .add(this.items)
      .search(get(this, this.searchParams.nodeservice));
  }),
});
