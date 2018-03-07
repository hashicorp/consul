import Route from '@ember/routing/route';
import { inject as service } from '@ember/service';
import { hash } from 'rsvp';

export default Route.extend({
  repo: service('dc'),
  model: function(/* params */) {
    const repo = this.get('repo');
    return hash({
      model: repo.findAll(),
    });
  },
  setupController: function(controller, model) {
    controller.setProperties(model);
  },
  afterModel: function({ model } /*, transition */) {
    // If we only have one datacenter, jump
    // straight to it and bypass the global
    // view
    if (model.get('length') === 1) {
      this.transitionTo('dc.services', model.get('firstObject').get('Name'));
    }
  },
});
