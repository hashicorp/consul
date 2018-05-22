import Controller from '@ember/controller';
import { computed, get } from '@ember/object';
import WithFiltering from 'consul-ui/mixins/with-filtering';
import ucfirst from 'consul-ui/utils/ucfirst';
import numeral from 'numeral';
// TODO: DRY out in acls at least
const createCounter = function(prop) {
  return function(items, val) {
    return val === '' ? get(items, 'length') : items.filterBy(prop, val).length;
  };
};
const countAction = createCounter('Action');
export default Controller.extend(WithFiltering, {
  queryParams: {
    action: {
      as: 'action',
    },
    s: {
      as: 'filter',
      replace: true,
    },
  },
  actionFilters: computed('items', function() {
    const items = get(this, 'items');
    return ['', 'allow', 'deny'].map(function(item) {
      return {
        label: `${item === '' ? 'All' : ucfirst(item)} (${numeral(
          countAction(items, item)
        ).format()})`,
        value: item,
      };
    });
  }),
  filter: function(item, { s = '', action = '' }) {
    return (
      (get(item, 'SourceName')
        .toLowerCase()
        .indexOf(s.toLowerCase()) !== -1 ||
        get(item, 'DestinationName')
          .toLowerCase()
          .indexOf(s.toLowerCase()) !== -1) &&
      (action === '' || get(item, 'Action') === action)
    );
  },
});
