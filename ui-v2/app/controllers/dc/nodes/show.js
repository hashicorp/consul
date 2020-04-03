import Controller from '@ember/controller';
import { inject as service } from '@ember/service';
import { get } from '@ember/object';
import { alias } from '@ember/object/computed';
import WithEventSource, { listen } from 'consul-ui/mixins/with-event-source';

export default Controller.extend(WithEventSource, {
  dom: service('dom'),
  notify: service('flashMessages'),
  items: alias('item.Services'),
  item: listen('item').catch(function(e) {
    if (e.target.readyState === 1) {
      // OPEN
      if (get(e, 'error.errors.firstObject.status') === '404') {
        this.notify.add({
          destroyOnClick: false,
          sticky: true,
          type: 'warning',
          action: 'update',
        });
        this.tomography.close();
        this.sessions.close();
      }
    }
  }),
  actions: {
    sortChecksByImportance: function(a, b) {
      const statusA = get(a, 'Status');
      const statusB = get(b, 'Status');
      switch (statusA) {
        case 'passing':
          // a = passing
          // unless b is also passing then a is less important
          return statusB === 'passing' ? 0 : 1;
        case 'critical':
          // a = critical
          // unless b is also critical then a is more important
          return statusB === 'critical' ? 0 : -1;
        case 'warning':
          // a = warning
          switch (statusB) {
            // b is passing so a is more important
            case 'passing':
              return -1;
            // b is critical so a is less important
            case 'critical':
              return 1;
            // a and b are both warning, therefore equal
            default:
              return 0;
          }
      }
      return 0;
    },
  },
});
