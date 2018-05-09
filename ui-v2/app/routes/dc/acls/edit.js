import Route from '@ember/routing/route';
import { inject as service } from '@ember/service';
import { hash } from 'rsvp';
import { get } from '@ember/object';

import WithAclActions from 'consul-ui/mixins/acl/with-actions';

export default Route.extend(WithAclActions, {
  repo: service('acls'),
  settings: service('settings'),
  model: function(params) {
    return hash({
      isLoading: false,
      item: get(this, 'repo').findBySlug(params.id, this.modelFor('dc').dc.Name),
      types: ['management', 'client'],
    });
  },
  setupController: function(controller, model) {
    this._super(...arguments);
    controller.setProperties(model);
  },
});
