import Route from '@ember/routing/route';
import { inject as service } from '@ember/service';
import { hash } from 'rsvp';

import WithBlockingActions from 'consul-ui/mixins/with-blocking-actions';

export default Route.extend(WithBlockingActions, {
  repo: service('repository/node'),
  sessionRepo: service('repository/session'),
  coordinateRepo: service('repository/coordinate'),
  queryParams: {
    s: {
      as: 'filter',
      replace: true,
    },
  },
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
  actions: {
    invalidateSession: function(item) {
      const dc = this.modelFor('dc').dc.Name;
      const nspace = this.modelFor('nspace').nspace.substr(1);
      const controller = this.controller;
      return this.feedback.execute(() => {
        return this.sessionRepo.remove(item).then(() => {
          return this.sessionRepo.findByNode(item.Node, dc, nspace).then(function(sessions) {
            controller.setProperties({
              sessions: sessions,
            });
          });
        });
      }, 'delete');
    },
  },
});
