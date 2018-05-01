import Mixin from '@ember/object/mixin';
import WithFiltering from 'consul-ui/mixins/with-filtering';
import { computed, get } from '@ember/object';
import ucfirst from 'consul-ui/utils/ucfirst';
import numeral from 'numeral';

const countStatus = function(items, status) {
  if (status === '') {
    return get(items, 'length');
  }
  const key = `Checks${ucfirst(status)}`;
  return items.reduce(function(prev, item, i, arr) {
    const num = get(item, key);
    return (
      prev +
        (typeof num !== 'undefined'
          ? num
          : get(item, 'Checks').filter(function(item) {
              return item.Status === status;
            }).length) || 0
    );
  }, 0);
};
export default Mixin.create(WithFiltering, {
  queryParams: {
    status: {
      as: 'status',
    },
    s: {
      as: 'filter',
    },
  },
  healthFilters: computed('items', function() {
    const items = get(this, 'items');
    const objs = ['', 'passing', 'warning', 'critical'].map(function(item) {
      const count = countStatus(items, item);
      return {
        count: count,
        label: `${item === '' ? 'All' : ucfirst(item)} (${numeral(count).format()})`,
        value: item,
      };
    });
    objs[0].label = `All (${numeral(
      objs.slice(1).reduce(function(prev, item, i, arr) {
        return prev + item.count;
      }, 0)
    ).format()})`;
    return objs;
  }),
});
