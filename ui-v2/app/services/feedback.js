import Service, { inject as service } from '@ember/service';
import { get, set } from '@ember/object';
import callableType from 'consul-ui/utils/callable-type';

const TYPE_SUCCESS = 'success';
const TYPE_ERROR = 'error';
export default Service.extend({
  notify: service('flashMessages'),
  logger: service('logger'),
  execute: function(handle, action, controller) {
    set(controller, 'isLoading', true);
    const getAction = callableType(action);
    const notify = get(this, 'notify');
    return (
      handle()
        //TODO: pass this through to getAction..
        .then(target => {
          notify.add({
            type: TYPE_SUCCESS,
            // here..
            action: getAction(),
            target: target,
          });
        })
        .catch(e => {
          get(this, 'logger').execute(e);
          if (e.name === 'TransitionAborted') {
            notify.add({
              type: TYPE_SUCCESS,
              // and here
              action: getAction(),
            });
          } else {
            notify.add({
              type: TYPE_ERROR,
              action: getAction(e),
            });
          }
        })
        .finally(function() {
          set(controller, 'isLoading', false);
        })
    );
  },
});
