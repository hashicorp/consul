import Component from '@ember/component';
import { computed } from '@ember/object';

export default Component.extend({
  // tagName: 'tbody',
  filter: '',
  filtered: computed('filter', 'items.@each', function() {
    const filter = this.get('filter');
    return this.get('items').filter(function(item) {
      return true;
      return item
        .get('filterKey')
        .toLowerCase()
        .match(filter.toLowerCase());
    });
  }),
});
