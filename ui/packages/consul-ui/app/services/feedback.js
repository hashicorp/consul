/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: MPL-2.0
 */

import Service, { inject as service } from '@ember/service';
import callableType from 'consul-ui/utils/callable-type';

const TYPE_SUCCESS = 'success';
const TYPE_ERROR = 'error';
const defaultStatus = function (type, obj) {
  return type;
};
const notificationDefaults = function () {
  return {
    timeout: 6000,
    extendedTimeout: 300,
    destroyOnClick: true,
  };
};
export default class FeedbackService extends Service {
  @service('flashMessages') notify;
  @service('logger') logger;

  notification(action, modelName) {
    return {
      success: (item) => this.success(item, action, undefined, modelName),
      error: (e) => this.error(e, action, undefined, modelName),
    };
  }

  success(item, action, status = defaultStatus, model) {
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
        model: model,
      });
    }
  }

  error(e, action, status = defaultStatus, model) {
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
        model: model,
      });
    } else {
      this.notify.add({
        ...notificationDefaults(),
        type: getStatus(TYPE_ERROR, e),
        action: getAction(),
        error: e,
        model: model,
      });
    }
  }

  async execute(handle, action, status, routeName) {
    let result;
    try {
      result = await handle();
      this.success(result, action, status, routeName);
    } catch (e) {
      this.error(e, action, status, routeName);
    }
  }
}
