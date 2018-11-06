import Controller from '@ember/controller';
import { get } from '@ember/object';
import { computed } from '@ember/object';
import sumOfUnhealthy from 'consul-ui/utils/sumOfUnhealthy';
import hasStatus from 'consul-ui/utils/hasStatus';
import WithHealthFiltering from 'consul-ui/mixins/with-health-filtering';
import WithSearching from 'consul-ui/mixins/with-searching';
export default Controller.extend(WithSearching, WithHealthFiltering, {
  init: function() {
    this.searchParams = {
      healthyServiceNode: 's',
      unhealthyServiceNode: 's',
    };
    this._super(...arguments);
  },
  searchableHealthy: computed('healthy', function() {
    return get(this, 'searchables.healthyServiceNode')
      .add(get(this, 'healthy'))
      .search(get(this, this.searchParams.healthyServiceNode));
  }),
  searchableUnhealthy: computed('unhealthy', function() {
    return get(this, 'searchables.unhealthyServiceNode')
      .add(get(this, 'unhealthy'))
      .search(get(this, this.searchParams.unhealthyServiceNode));
  }),
  unhealthy: computed('filtered', function() {
    return get(this, 'filtered').filter(function(item) {
      return sumOfUnhealthy(item.Checks) > 0;
    });
  }),
  healthy: computed('filtered', function() {
    return get(this, 'filtered').filter(function(item) {
      return sumOfUnhealthy(item.Checks) === 0;
    });
  }),
  filter: function(item, { s = '', status = '' }) {
    return hasStatus(get(item, 'Checks'), status);
  },
});
