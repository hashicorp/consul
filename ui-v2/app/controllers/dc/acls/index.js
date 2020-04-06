import Controller from '@ember/controller';
import { computed, get } from '@ember/object';
import WithFiltering from 'consul-ui/mixins/with-filtering';
import WithSearching from 'consul-ui/mixins/with-searching';
import ucfirst from 'consul-ui/utils/ucfirst';
const countType = function(items, type) {
  return type === '' ? get(items, 'length') : items.filterBy('Type', type).length;
};
export default Controller.extend(WithSearching, WithFiltering, {
  queryParams: {
    type: {
      as: 'type',
    },
    s: {
      as: 'filter',
      replace: true,
    },
  },
  init: function() {
    this.searchParams = {
      acl: 's',
    };
    this._super(...arguments);
  },
  searchable: computed('filtered', function() {
    return get(this, 'searchables.acl')
      .add(get(this, 'filtered'))
      .search(get(this, this.searchParams.acl));
  }),
  typeFilters: computed('items', function() {
    const items = get(this, 'items');
    return ['', 'management', 'client'].map(function(item) {
      return {
        label: `${item === '' ? 'All' : ucfirst(item)} (${countType(
          items,
          item
        ).toLocaleString()})`,
        value: item,
      };
    });
  }),
  filter: function(item, { type = '' }) {
    return type === '' || get(item, 'Type') === type;
  },
  actions: {
    sendClone: function(item) {
      this.send('clone', item);
    },
  },
});
