import Route from '@ember/routing/route';
import { inject as service } from '@ember/service';

import { hash } from 'rsvp';

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
    // Return a promise hash of the node
    return hash({
      dc: dc.dc,
      tomography: tomography(params.name, dc),
      node: repo.findBySlug(params.name, dc.dc),
    });
  },
  // Load the sessions for the node
  // jc: any reason why this is in afterModel?
  afterModel: function(models) {
    const sessionRepo = this.get('sessionRepo');
    return sessionRepo.findByNode(models.node.Node, models.dc).then(function(data) {
      models.sessions = data;
      return models;
    });
  },
  setupController: function(controller, models) {
    controller.set('model', models.node);
    controller.set('sessions', models.sessions);
    controller.set('tomography', models.tomography);
    controller.set('size', 337);
  },
});
