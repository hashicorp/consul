import Route from '@ember/routing/route';
import { inject as service } from '@ember/service';

export default Route.extend({
  repo: service('services'),
  model: function(params) {
    // Return a promise to retrieve all of the services
    return this.get('repo').findAllByDatacenter(this.modelFor('dc').dc);
  },
  setupController: function(controller, model) {
    controller.set('services', model);
  },
});
