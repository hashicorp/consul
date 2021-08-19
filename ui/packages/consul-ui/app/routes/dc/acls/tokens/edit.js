import { inject as service } from '@ember/service';
import SingleRoute from 'consul-ui/routing/single';
import { hash } from 'rsvp';

import WithTokenActions from 'consul-ui/mixins/token/with-actions';

export default class EditRoute extends SingleRoute.extend(WithTokenActions) {
  @service('repository/token') repo;
  @service('settings') settings;

  model(params, transition) {
    return super.model(...arguments).then(model => {
      return hash({
        ...model,
        ...{
          token: this.settings.findBySlug('token'),
        },
      });
    });
  }

  setupController(controller, model) {
    super.setupController(...arguments);
    controller.setProperties(model);
  }
}
