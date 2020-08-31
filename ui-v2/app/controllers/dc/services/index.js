import Controller from '@ember/controller';
import { computed } from '@ember/object';

export default Controller.extend({
  queryParams: {
    sortBy: 'sort',
    search: {
      as: 'filter',
    },
  },
  services: computed('items.[]', function() {
    return this.items.filter(function(item) {
      return item.Kind !== 'connect-proxy';
    });
  }),
});
