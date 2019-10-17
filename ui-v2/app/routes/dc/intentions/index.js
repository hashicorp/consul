import Route from '@ember/routing/route';
import { inject as service } from '@ember/service';
import { hash } from 'rsvp';

import WithIntentionActions from 'consul-ui/mixins/intention/with-actions';

export default Route.extend(WithIntentionActions, {
  repo: service('repository/intention'),
  queryParams: {
    s: {
      as: 'filter',
      replace: true,
    },
  },
  model: function(params) {
    return hash({
      items: this.repo.findAllByDatacenter(
        this.modelFor('dc').dc.Name,
        this.modelFor('nspace').nspace.substr(1)
      ),
    });
  },
  setupController: function(controller, model) {
    controller.setProperties(model);
  },
});
