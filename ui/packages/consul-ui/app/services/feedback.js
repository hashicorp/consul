import Service, { inject as service } from '@ember/service';
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
export default class FeedbackService extends Service {
  @service('flashMessages')
  notify;

  @service('logger')
  logger;

  execute(handle, action, status = defaultStatus, controller) {
    const getAction = callableType(action);
    const getStatus = callableType(status);
    const notify = this.notify;
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
          this.logger.execute(e);
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
              error: e,
            });
          }
        })
    );
  }
}
