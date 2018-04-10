import Route from '@ember/routing/route';
import { inject as service } from '@ember/service';
import { hash } from 'rsvp';

export default Route.extend({
  repo: service('nodes'),
  model: function(params) {
    return hash({
      items: this.get('repo').findAllByDatacenter(this.modelFor('dc').dc),
    });
  },
  setupController: function(controller, model) {
    controller.setProperties(model);
  },
});
