import Route from '@ember/routing/route';
import { inject as service } from '@ember/service';
import { hash } from 'rsvp';
import { get } from '@ember/object';

import WithAclActions from 'consul-ui/mixins/intention/with-actions';

export default Route.extend(WithAclActions, {
  repo: service('intentions'),
  model: function(params) {
    return hash({
      isLoading: false,
      item: get(this, 'repo').findBySlug(params.id, this.modelFor('dc').dc.Name),
      types: ['consul', 'externaluri'],
      intents: ['allow', 'deny'],
    });
  },
  setupController: function(controller, model) {
    this._super(...arguments);
    controller.setProperties(model);
  },
});
