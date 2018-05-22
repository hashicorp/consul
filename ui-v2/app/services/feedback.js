import Service, { inject as service } from '@ember/service';
import { get, set } from '@ember/object';

export default Service.extend({
  notify: service('flashMessages'),
  logger: service('logger'),
  execute: function(handle, success, error, controller) {
    set(controller, 'isLoading', true);
    const notify = get(this, 'notify');
    return handle()
      .then(() => {
        notify.add({
          type: 'success',
          message: success,
        });
      })
      .catch(e => {
        get(this, 'logger').execute(e);
        if (e.name === 'TransitionAborted') {
          notify.add({
            type: 'success',
            message: success,
          });
        } else {
          notify.add({
            type: 'error',
            message: error,
          });
        }
      })
      .finally(function() {
        set(controller, 'isLoading', false);
      });
  },
});
