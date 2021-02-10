import { inject as service } from '@ember/service';
import Route from 'consul-ui/routing/route';
import { hash } from 'rsvp';

import WithAclActions from 'consul-ui/mixins/acl/with-actions';

export default class EditRoute extends Route.extend(WithAclActions) {
  @service('repository/acl')
  repo;

  @service('settings')
  settings;

  model(params) {
    return hash({
      item: this.repo.findBySlug({
        dc: this.modelFor('dc').dc.Name,
        id: params.id,
      }),
      types: ['management', 'client'],
    });
  }

  setupController(controller, model) {
    super.setupController(...arguments);
    controller.setProperties(model);
  }
}
