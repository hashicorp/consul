import Route from '@ember/routing/route';
import { inject as service } from '@ember/service';
import { hash } from 'rsvp';
import { get } from '@ember/object';

export default Route.extend({
  repo: service('dc'),
  settings: service('settings'),
  model: function(params) {
    const repo = get(this, 'repo');
    return repo.findAll().then(function(items) {
      return hash({
        dcs: items,
        dc: repo.findBySlug(params.dc, items),
      });
    });
  },
  setupController: function(controller, model) {
    controller.setProperties(model);
  },
});
