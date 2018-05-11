import Service from '@ember/service';
import { inject as service } from '@ember/service';
import { get } from '@ember/object';

export default Service.extend({
  // TODO: Why can't I name this `notify`?
  flashMessages: service('flashMessages'),
  execute: function(handle, success, error) {
    const controller = this.controller;
    controller.set('isLoading', true);
    return handle()
      .then(() => {
        get(this, 'flashMessages').add({
          type: 'success',
          message: success,
        });
      })
      .catch(e => {
        if (e.name === 'TransitionAborted') {
          get(this, 'flashMessages').add({
            type: 'success',
            message: success,
          });
        } else {
          get(this, 'flashMessages').add({
            type: 'error',
            message: error,
          });
        }
      })
      .finally(function() {
        controller.set('isLoading', false);
      });
  },
});
