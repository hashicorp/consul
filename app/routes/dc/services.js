import Route from '@ember/routing/route';
import { inject as service } from '@ember/service';
import { hash } from 'rsvp';

export default Route.extend({
  repo: service('services'),
  model: function(/* params */) {
    const repo = this.get('repo');
    return hash({
      services: repo.findAllByDatacenter(this.paramsFor('dc').dc),
    });
  },
  setupController: function(controller, model) {
    controller.setProperties(model);
  },
});
