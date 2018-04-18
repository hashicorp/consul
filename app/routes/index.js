import Route from '@ember/routing/route';
import { inject as service } from '@ember/service';
import { hash } from 'rsvp';
import { get } from '@ember/object';

export default Route.extend({
  repo: service('dc'),
  model: function(params) {
    const repo = get(this, 'repo');
    return hash({
      items: repo.findAll(),
    });
  },
  setupController: function(controller, model) {
    controller.setProperties(model);
  },
  afterModel: function({ items }, transition) {
    // If we only have one datacenter, jump
    // straight to it and bypass the global
    // view
    if (get(items, 'length') === 1) {
      this.transitionTo('dc.services', get(items, 'firstObject.Name'));
    }
  },
});
