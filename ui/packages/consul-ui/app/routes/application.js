import Route from 'consul-ui/routing/route';
import { action } from '@ember/object';

import WithBlockingActions from 'consul-ui/mixins/with-blocking-actions';

export default class ApplicationRoute extends Route.extend(WithBlockingActions) {
  @action
  error(e, transition) {
    // TODO: Normalize all this better
    let error = {
      status: e.code || e.statusCode || '',
      message: e.message || e.detail || 'Error',
    };
    if (e.errors && e.errors[0]) {
      error = e.errors[0];
      error.message = error.message || error.title || error.detail || 'Error';
    }
    if (error.status === '') {
      error.message = 'Error';
    }
    this.controllerFor('application').setProperties({ error: error });
    return true;
  }
}
