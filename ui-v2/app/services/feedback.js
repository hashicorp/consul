import Service, { inject as service } from '@ember/service';
import { get, set } from '@ember/object';
import callableType from 'consul-ui/utils/callable-type';

const TYPE_SUCCESS = 'success';
const TYPE_ERROR = 'error';
const defaultStatus = function(type, obj) {
  return type;
};
const notificationDefaults = function() {
  return {
    timeout: 6000,
    extendedTimeout: 300,
  };
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
        .then(item => {
          // returning exactly `false` for a feedback action means even though
          // its successful, please skip this notification and don't display it
          if (item !== false) {
            notify.clearMessages();
            // TODO right now the majority of `item` is a Transition
            // but you can resolve an object
            notify.add({
              ...notificationDefaults(),
              type: getStatus(TYPE_SUCCESS),
              // here..
              action: getAction(),
              item: item,
            });
          }
        })
        .catch(e => {
          notify.clearMessages();
          get(this, 'logger').execute(e);
          if (e.name === 'TransitionAborted') {
            notify.add({
              ...notificationDefaults(),
              type: getStatus(TYPE_SUCCESS),
              // and here
              action: getAction(),
            });
          } else {
            notify.add({
              ...notificationDefaults(),
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
