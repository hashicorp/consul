import Controller from '@ember/controller';
import WithEventSource from 'consul-ui/mixins/with-event-source';

export default Controller.extend(WithEventSource, {
  queryParams: {
    filterBy: {
      as: 'status',
    },
    search: {
      as: 'filter',
      replace: true,
    },
  },
  actions: {
    hasStatus: function(status, checks) {
      if (status === '') {
        return true;
      }
      return checks.some(item => item.Status === status);
    },
    isHealthy: function(checks) {
      return !this.actions.isUnhealthy.apply(this, [checks]);
    },
    isUnhealthy: function(checks) {
      return checks.some(item => item.Status === 'critical' || item.Status === 'warning');
    },
  },
});
