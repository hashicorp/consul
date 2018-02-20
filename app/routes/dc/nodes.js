import Route from '@ember/routing/route';

import { inject as service } from '@ember/service';

export default Route.extend({
  repo: service('nodes'),
  model: function(params) {
    // Return a promise containing the nodes
    return this.get('repo').findAllByDatacenter(this.modelFor('dc').dc);
  },
  setupController: function(controller, model) {
    controller.set('nodes', model);
  },
});
