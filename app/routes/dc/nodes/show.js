import Route from '@ember/routing/route';
import { inject as service } from '@ember/service';
import { hash } from 'rsvp';
import { assign } from '@ember/polyfills';

import tomography from 'consul-ui/utils/tomography';

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
      tomography: tomography(params.name, dc),
      model: repo.findBySlug(params.name, dc.dc),
      size: 337,
    }).then(function(model) {
      // Load the sessions for the node
      // jc: This was in afterModel, I think the only for which was
      // that the model needed resolving first to get to Node
      return assign(model, {
        sessions: sessionRepo.findByNode(model.model.get('Node'), model.dc),
      });
    });
  },
  setupController: function(controller, model) {
    controller.setProperties(model);
  },
});
