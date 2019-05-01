import SingleRoute from 'consul-ui/routing/single';
import { inject as service } from '@ember/service';
import { hash } from 'rsvp';
import { get } from '@ember/object';

import WithTokenActions from 'consul-ui/mixins/token/with-actions';

export default SingleRoute.extend(WithTokenActions, {
  repo: service('repository/token'),
  settings: service('settings'),
  model: function(params, transition) {
    return this._super(...arguments).then(model => {
      return hash({
        ...model,
        ...{
          token: get(this, 'settings').findBySlug('token'),
        },
      });
    });
  },
  setupController: function(controller, model) {
    controller.setProperties(model);
  },
});
