import Controller from '@ember/controller';
import { computed } from '@ember/object';
import WithHealthFiltering from 'consul-ui/mixins/with-health-filtering';
import { get } from '@ember/object';
export default Controller.extend(WithHealthFiltering, {
  init: function() {
    this._super(...arguments);
    this.columns = [25, 25, 25, 25];
  },
  unhealthy: computed('filtered', function() {
    return get(this, 'filtered').filter(function(item) {
      return get(item, 'isUnhealthy');
    });
  }),
  healthy: computed('filtered', function() {
    return get(this, 'filtered').filter(function(item) {
      return get(item, 'isHealthy');
    });
  }),
  filter: function(item, { s = '', status = '' }) {
    return (
      get(item, 'Node')
        .toLowerCase()
        .indexOf(s.toLowerCase()) !== -1 && item.hasStatus(status)
    );
  },
});
