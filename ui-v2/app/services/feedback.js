import Service, { inject as service } from '@ember/service';
import { get, set } from '@ember/object';
import callableType from 'consul-ui/utils/callable-type';

export default Service.extend({
  notify: service('flashMessages'),
  logger: service('logger'),
  execute: function(handle, success, error, controller) {
    set(controller, 'isLoading', true);
    const displaySuccess = callableType(success);
    const displayError = callableType(error);
    const notify = get(this, 'notify');
    return (
      handle()
        //TODO: pass this through to display success..
        .then(() => {
          notify.add({
            type: 'success',
            // here..
            message: displaySuccess(),
          });
        })
        .catch(e => {
          get(this, 'logger').execute(e);
          if (e.name === 'TransitionAborted') {
            notify.add({
              type: 'success',
              // and here
              message: displaySuccess(),
            });
          } else {
            notify.add({
              type: 'error',
              message: displayError(e),
            });
          }
        })
        .finally(function() {
          set(controller, 'isLoading', false);
        })
    );
  },
});
