import Controller from '@ember/controller';
import { computed } from '@ember/object';

export default Controller.extend({
  queryParams: {
    sortBy: 'sort',
    status: 'status',
    source: 'source',
    type: 'type',
    search: {
      as: 'filter',
    },
  },
  services: computed('items.[]', function() {
    return this.items.filter(function(item) {
      return item.Kind !== 'connect-proxy';
    });
  }),
  externalSources: computed('services', function() {
    const sources = this.services.reduce(function(prev, item) {
      return prev.concat(item.ExternalSources || []);
    }, []);
    // unique, non-empty values, alpha sort
    return [...new Set(sources)].filter(Boolean).sort();
  }),
});
