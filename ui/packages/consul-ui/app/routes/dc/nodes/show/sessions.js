import Route from 'consul-ui/routing/route';
import { inject as service } from '@ember/service';
import { action } from '@ember/object';

import WithBlockingActions from 'consul-ui/mixins/with-blocking-actions';

export default class SessionsRoute extends Route.extend(WithBlockingActions) {
  @service('repository/session') sessionRepo;
  @service('feedback') feedback;

  @action
  invalidateSession(item) {
    const route = this;
    return this.feedback.execute(() => {
      return this.sessionRepo.remove(item).then(() => {
        route.refresh();
      });
    }, 'delete');
  }
}
