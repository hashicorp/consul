import Route from '@ember/routing/route';
import { hash } from 'rsvp';

import get from 'consul-ui/utils/request/get';
import tomography from 'consul-ui/utils/tomography';
import Node from 'consul-ui/models/dc/node';

export default Route.extend({
  queryParams: {
    filter: {
      replace: true,
      as: 'other-filter',
    },
  },
  model: function(params) {
    var dc = this.modelFor('dc');
    // Return a promise hash of the node
    return hash({
      dc: dc.dc,
      tomography: tomography(params.name, dc),
      node: get('/v1/internal/ui/node/' + params.name, dc.dc).then(function(data) {
        return Node.create(data);
      }),
    });
  },
  // Load the sessions for the node
  afterModel: function(models) {
    return get('/v1/session/node/' + models.node.Node, models.dc).then(function(data) {
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
