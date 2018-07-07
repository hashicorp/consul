import Controller from '@ember/controller';
import { computed, get } from '@ember/object';
import WithFiltering from 'consul-ui/mixins/with-filtering';
import ucfirst from 'consul-ui/utils/ucfirst';
import numeral from 'numeral';
const countType = function(items, type) {
  return type === '' ? get(items, 'length') : items.filterBy('Type', type).length;
};
export default Controller.extend(WithFiltering, {
  queryParams: {
    type: {
      as: 'type',
    },
    s: {
      as: 'filter',
      replace: true,
    },
  },
  typeFilters: computed('items', function() {
    const items = get(this, 'items');
    return ['', 'management', 'client'].map(function(item) {
      return {
        label: `${item === '' ? 'All' : ucfirst(item)} (${numeral(
          countType(items, item)
        ).format()})`,
        value: item,
      };
    });
  }),
  filter: function(item, { s = '', type = '' }) {
    const sLower = s.toLowerCase();
    return (
      (get(item, 'Name')
        .toLowerCase()
        .indexOf(sLower) !== -1 ||
        get(item, 'ID')
          .toLowerCase()
          .indexOf(sLower) !== -1) &&
      (type === '' || get(item, 'Type') === type)
    );
  },
  actions: {
    sendClone: function(item) {
      this.send('clone', item);
    },
  },
});
