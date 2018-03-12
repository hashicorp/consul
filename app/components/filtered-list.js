import Component from '@ember/component';
import { computed } from '@ember/object';

export default Component.extend({
  filter: '',
  filtered: computed('filter', 'items.@each', function() {
    const filter = this.get('filter');
    return this.get('items').filter(function(item) {
      return item
        .get('filterKey')
        .toLowerCase()
        .match(filter.toLowerCase());
    });
  }),
});
