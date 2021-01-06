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
  @service('flashMessages') notify;
  @service('logger') logger;

  notification(action) {
    return {
      success: item => this.success(item, action),
      error: e => this.error(e, action),
    };
  }

  success(item, action, status = defaultStatus) {
    const getAction = callableType(action);
    const getStatus = callableType(status);
    // returning exactly `false` for a feedback action means even though
    // its successful, please skip this notification and don't display it
    if (item !== false) {
      this.notify.clearMessages();
      // TODO right now the majority of `item` is a Transition
      // but you can resolve an object
      this.notify.add({
        ...notificationDefaults(),
        type: getStatus(TYPE_SUCCESS),
        // here..
        action: getAction(),
        item: item,
      });
    }
  }

  error(e, action, status = defaultStatus) {
    const getAction = callableType(action);
    const getStatus = callableType(status);
    this.notify.clearMessages();
    this.logger.execute(e);
    if (e.name === 'TransitionAborted') {
      this.notify.add({
        ...notificationDefaults(),
        type: getStatus(TYPE_SUCCESS),
        // and here
        action: getAction(),
      });
    } else {
      this.notify.add({
        ...notificationDefaults(),
        type: getStatus(TYPE_ERROR, e),
        action: getAction(),
        error: e,
      });
    }
  }

  async execute(handle, action, status) {
    let result;
    try {
      result = await handle();
      this.success(result, action, status);
    } catch (e) {
      this.error(e, action, status);
    }
  }
}
