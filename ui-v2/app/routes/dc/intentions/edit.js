import Route from '@ember/routing/route';
import { inject as service } from '@ember/service';
import { hash } from 'rsvp';

export default Route.extend({
  repo: service('repository/intention'),
  model: function(params, transition) {
    const dc = this.modelFor('dc').dc.Name;
    const nspace = '*';
    return hash({
      isLoading: false,
      dc: dc,
      nspace: nspace,
      item:
        typeof params.id !== 'undefined' ? this.repo.findBySlug(params.id, dc, nspace) : undefined,
    });
  },
  setupController: function(controller, model) {
    controller.setProperties(model);
  },
});
