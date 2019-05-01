import Route from '@ember/routing/route';
import { inject as service } from '@ember/service';
import { hash } from 'rsvp';
import { get } from '@ember/object';
export default Route.extend({
  repo: service('repository/dc'),
  settings: service('settings'),
  model: function(params) {
    const repo = get(this, 'repo');
    return hash({
      dcs: repo.findAll(),
    }).then(function(model) {
      return hash({
        ...model,
        ...{
          dc: repo.findBySlug(params.dc, model.dcs),
        },
      });
    });
  },
  setupController: function(controller, model) {
    controller.setProperties(model);
  },
});
