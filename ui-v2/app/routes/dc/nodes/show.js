import Route from '@ember/routing/route';
import { inject as service } from '@ember/service';
import { hash } from 'rsvp';

export default Route.extend({
  repo: service('repository/node'),
  sessionRepo: service('repository/session'),
  coordinateRepo: service('repository/coordinate'),
  model: function(params) {
    const dc = this.modelFor('dc').dc.Name;
    const nspace = this.modelFor('nspace').nspace.substr(1);
    const name = params.name;
    return hash({
      item: this.repo.findBySlug(name, dc, nspace),
      sessions: this.sessionRepo.findByNode(name, dc, nspace),
      tomography: this.coordinateRepo.findAllByNode(name, dc),
    });
  },
  setupController: function(controller, model) {
    controller.setProperties(model);
  },
});
