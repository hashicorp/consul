import Controller from '@ember/controller';
import { computed } from '@ember/object';
import WithHealthFiltering from 'consul-ui/mixins/with-health-filtering';
export default Controller.extend(WithHealthFiltering, {
  columns: [25, 25, 25, 25],
  unhealthy: computed('filtered', function() {
    return this.get('filtered').filter(function(item) {
      return item.get('isUnhealthy');
    });
  }),
  healthy: computed('filtered', function() {
    return this.get('filtered').filter(function(item) {
      return item.get('isHealthy');
    });
  }),
  filter: function(item, { s = '', status = '' }) {
    return (
      item
        .get('Node')
        .toLowerCase()
        .indexOf(s.toLowerCase()) === 0 && item.hasStatus(status)
    );
  },
});
