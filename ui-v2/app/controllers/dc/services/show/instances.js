import Controller from '@ember/controller';
import { computed } from '@ember/object';

export default Controller.extend({
  queryParams: {
    sortBy: 'sort',
    status: 'status',
    source: 'source',
    search: {
      as: 'filter',
      replace: true,
    },
  },
  externalSources: computed('items', function() {
    const sources = this.items.reduce(function(prev, item) {
      return prev.concat(item.ExternalSources || []);
    }, []);
    // unique, non-empty values, alpha sort
    return [...new Set(sources)].filter(Boolean).sort();
  }),
});
