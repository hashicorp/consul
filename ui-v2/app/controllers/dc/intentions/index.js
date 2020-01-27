import Controller from '@ember/controller';
import { computed, get } from '@ember/object';
import WithFiltering from 'consul-ui/mixins/with-filtering';
import WithSearching from 'consul-ui/mixins/with-searching';
import ucfirst from 'consul-ui/utils/ucfirst';
// TODO: DRY out in acls at least
const createCounter = function(prop) {
  return function(items, val) {
    return val === '' ? get(items, 'length') : items.filterBy(prop, val).length;
  };
};
const countAction = createCounter('Action');
export default Controller.extend(WithSearching, WithFiltering, {
  queryParams: {
    action: {
      as: 'action',
    },
    s: {
      as: 'filter',
      replace: true,
    },
  },
  init: function() {
    this.searchParams = {
      intention: 's',
    };
    this._super(...arguments);
  },
  searchable: computed('filtered', function() {
    return get(this, 'searchables.intention')
      .add(get(this, 'filtered'))
      .search(get(this, this.searchParams.intention));
  }),
  actionFilters: computed('items', function() {
    const items = get(this, 'items');
    return ['', 'allow', 'deny'].map(function(item) {
      return {
        label: `${item === '' ? 'All' : ucfirst(item)} (${countAction(
          items,
          item
        ).toLocaleString()})`,
        value: item,
      };
    });
  }),
  filter: function(item, { s = '', action = '' }) {
    return action === '' || get(item, 'Action') === action;
  },
});
