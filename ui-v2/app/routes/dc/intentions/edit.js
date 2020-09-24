import Route from '@ember/routing/route';
import { inject as service } from '@ember/service';
import { hash } from 'rsvp';

export default Route.extend({
  repo: service('repository/intention'),
  model: function({ intention_id }, transition) {
    const dc = this.modelFor('dc').dc.Name;
    const nspace = '*';
    return hash({
      routeName: this.routeName,
      dc: dc,
      nspace: nspace,
      item:
        typeof intention_id !== 'undefined'
          ? this.repo.findBySlug(intention_id, dc, nspace)
          : this.repo.create({
              Datacenter: dc,
            }),
    });
  },
  setupController: function(controller, model) {
    controller.setProperties(model);
  },
});
