import Route from '@ember/routing/route';
import { inject as service } from '@ember/service';

export default Route.extend({
  repo: service('settings'),
  model: function(params) {
    return {
      model: {
        token: this.get('repo').get('token'),
      },
    };
  },
  setupController: function(controller, model) {
    controller.setProperties(model);
  },
});
