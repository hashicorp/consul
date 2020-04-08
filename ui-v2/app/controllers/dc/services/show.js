import Controller from '@ember/controller';
import { get } from '@ember/object';
import { inject as service } from '@ember/service';
import WithEventSource, { listen } from 'consul-ui/mixins/with-event-source';
export default Controller.extend(WithEventSource, {
  dom: service('dom'),
  notify: service('flashMessages'),
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
      }
    }
  }),
});
