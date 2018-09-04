import Route from '@ember/routing/route';
import { inject as service } from '@ember/service';
import { hash } from 'rsvp';
import { get } from '@ember/object';

import WithTokenActions from 'consul-ui/mixins/token/with-actions';

export default Route.extend(WithTokenActions, {
  repo: service('tokens'),
  settings: service('settings'),
  model: function(params) {
    return hash({
      isLoading: false,
      item: get(this, 'repo').findBySlug(params.id, this.modelFor('dc').dc.Name),
    });
  },
  setupController: function(controller, model) {
    this._super(...arguments);
    controller.setProperties(model);
  },
});
