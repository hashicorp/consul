import Controller from '@ember/controller';
import { get } from '@ember/object';
import { inject as service } from '@ember/service';
export default Controller.extend({
  dom: service('dom'),
  notify: service('flashMessages'),
  actions: {
    error: function(e) {
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
        // any other potential eventsources should be cancelled here
        // hence the loop to make it easier to add if need be
        [this.intentions].forEach(function(item) {
          if (typeof item.close === 'function') {
            item.close();
          }
        });
      }
    },
  },
});
