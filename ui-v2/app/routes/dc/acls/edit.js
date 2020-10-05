import Route from 'consul-ui/routing/route';
import { inject as service } from '@ember/service';
import { hash } from 'rsvp';

import WithAclActions from 'consul-ui/mixins/acl/with-actions';

export default Route.extend(WithAclActions, {
  repo: service('repository/acl'),
  settings: service('settings'),
  model: function(params) {
    return hash({
      item: this.repo.findBySlug(params.id, this.modelFor('dc').dc.Name),
      types: ['management', 'client'],
    });
  },
  setupController: function(controller, model) {
    this._super(...arguments);
    controller.setProperties(model);
  },
});
