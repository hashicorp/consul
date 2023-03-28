import { inject as service } from '@ember/service';
import Route from 'consul-ui/routing/route';

import WithBlockingActions from 'consul-ui/mixins/with-blocking-actions';

export default class EditRoute extends Route.extend(WithBlockingActions) {
  @service('repository/token') repo;
  @service('settings') settings;

  async model(params, transition) {
    return {
      token: await this.settings.findBySlug('token'),
    };
  }
}
