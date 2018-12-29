import Service, { inject as service } from '@ember/service';
import { get, set } from '@ember/object';
import callableType from 'consul-ui/utils/callable-type';

const TYPE_SUCCESS = 'success';
const TYPE_ERROR = 'error';
const defaultStatus = function(type, obj) {
  return type;
};
export default Service.extend({
  notify: service('flashMessages'),
  logger: service('logger'),
  execute: function(handle, action, status = defaultStatus, controller) {
    set(controller, 'isLoading', true);
    const getAction = callableType(action);
    const getStatus = callableType(status);
    const notify = get(this, 'notify');
    return (
      handle()
        //TODO: pass this through to getAction..
        .then(target => {
          notify.add({
            type: getStatus(TYPE_SUCCESS),
            // here..
            action: getAction(),
          });
        })
        .catch(e => {
          get(this, 'logger').execute(e);
          if (e.name === 'TransitionAborted') {
            notify.add({
              type: getStatus(TYPE_SUCCESS),
              // and here
              action: getAction(),
            });
          } else {
            notify.add({
              type: getStatus(TYPE_ERROR, e),
              action: getAction(),
            });
          }
        })
        .finally(function() {
          set(controller, 'isLoading', false);
        })
    );
  },
});
