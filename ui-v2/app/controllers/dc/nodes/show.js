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
});
