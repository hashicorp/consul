import Controller from '@ember/controller';
import { computed, get } from '@ember/object';
import WithFiltering from 'consul-ui/mixins/with-filtering';
import ucfirst from 'consul-ui/utils/ucfirst';
import humanize from 'consul-ui/utils/humanize';
const countType = function(items, type) {
  return type === '' ? get(items, 'length') : items.filterBy('Type', type).length;
};
export default Controller.extend(WithFiltering, {
  init: function() {
    this._super(...arguments);
    this.filters = {
      type: '',
    };
  },
  typeFilters: computed('items', function() {
    const items = get(this, 'items');
    return ['', 'management', 'client'].map(function(item) {
      return {
        label: `${item === '' ? 'All' : ucfirst(item)} (${humanize(countType(items, item))})`,
        value: item,
      };
    });
  }),
  filter: function(item, { s = '', type = '' }) {
    return (
      item
        .get('Name')
        .toLowerCase()
        .indexOf(s.toLowerCase()) === 0 &&
      (type === '' || item.get('Type') === type)
    );
  },
});
