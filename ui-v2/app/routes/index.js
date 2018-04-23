import Route from '@ember/routing/route';
import { inject as service } from '@ember/service';
import { hash } from 'rsvp';
import { get } from '@ember/object';

export default Route.extend({
  repo: service('dc'),
  settings: service('settings'),
  model: function(params) {
    const repo = get(this, 'repo');
    return hash({
      dc: get(this, 'settings').findBySlug('dc'),
      items: repo.findAll(),
    });
  },
  setupController: function(controller, model) {
    controller.setProperties(model);
  },
  afterModel: function({ dc, items }, transition) {
    // if we don't have a previous DC saved,
    // go to the first DC from the list (which is alphabetically sorted)
    if (dc == null) {
      dc = get(items, 'firstObject.Name');
    }
    this.transitionTo('dc.services', dc);
  },
});
