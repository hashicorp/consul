import Controller from '@ember/controller';
import { get, computed } from '@ember/object';
import WithHealthFiltering from 'consul-ui/mixins/with-health-filtering';
import WithSearching from 'consul-ui/mixins/with-searching';

export default Controller.extend(WithSearching, WithHealthFiltering, {
  init: function() {
    this.searchParams = {
      healthyNode: 's',
      unhealthyNode: 's',
    };
    this._super(...arguments);
  },
  searchableHealthy: computed('healthy', function() {
    return get(this, 'searchables.healthyNode')
      .add(this.healthy)
      .search(get(this, this.searchParams.healthyNode));
  }),
  searchableUnhealthy: computed('unhealthy', function() {
    return get(this, 'searchables.unhealthyNode')
      .add(this.unhealthy)
      .search(get(this, this.searchParams.unhealthyNode));
  }),
  unhealthy: computed('filtered', function() {
    return this.filtered.filter(function(item) {
      return get(item, 'isUnhealthy');
    });
  }),
  healthy: computed('filtered', function() {
    return this.filtered.filter(function(item) {
      return get(item, 'isHealthy');
    });
  }),
  filter: function(item, { s = '', status = '' }) {
    return item.hasStatus(status);
  },
});
