import Route from '@ember/routing/route';
import { inject as service } from '@ember/service';
import { hash } from 'rsvp';
import { assign } from '@ember/polyfills';

export default Route.extend({
  repo: service('nodes'),
  sessionRepo: service('session'),
  queryParams: {
    filter: {
      replace: true,
      as: 'other-filter',
    },
  },
  model: function(params) {
    const dc = this.modelFor('dc');
    const repo = this.get('repo');
    const sessionRepo = this.get('sessionRepo');
    // Return a promise hash of the node
    return hash({
      dc: dc.dc,
      // TODO: bring this in
      // tomography: tomography(params.name, dc),
      model: repo.findBySlug(params.name, dc.dc),
      size: 337,
    }).then(function(model) {
      // Load the sessions for the node
      // jc: This was in afterModel, I think the only for which was
      // that the model needed resolving first to get to Node
      return hash(
        assign({}, model, {
          sessions: sessionRepo.findByNode(model.model.get('Node'), model.dc),
        })
      );
    });
  },
  setupController: function(controller, model) {
    controller.setProperties(model);
  },
  actions: {
    // TODO: use feedback service
    invalidateSession: function(session) {
      const controller = this.controller;
      controller.set('isLoading', true);
      const dc = this.modelFor('dc').dc;
      // Delete the session
      const sessionRepo = this.get('sessionRepo');
      sessionRepo
        .remove(session, dc)
        .then(() => {
          const node = controller.get('model');
          return sessionRepo.findByNode(node.get('Node'), dc).then(function(sessions) {
            controller.set('sessions', sessions);
          });
        })
        .catch(function(e) {
          // TODO: Make sure errors are dealt with properly
          // Render the error message on the form if the request failed
          controller.set('errorMessage', 'Received error while processing: ' + e.statusText);
        })
        .finally(function() {
          controller.set('isLoading', false);
        });
    },
  },
});
