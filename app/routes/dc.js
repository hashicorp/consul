import Route from '@ember/routing/route';
import { hash } from 'rsvp';

import get from 'consul-ui/utils/request/get';
import map from 'consul-ui/utils/map';

import Node from 'consul-ui/models/dc/node';
export default Route.extend({
  model: function(params) {
    // Return a promise hash to retreieve the
    // dcs and nodes used in the header
    return hash({
      dc: params.dc,
      dcs: get('/v1/catalog/datacenters'),
      nodes: get('/v1/internal/ui/nodes', params.dc).then(map(Node)),
      coordinates: get('/v1/coordinate/nodes', params.dc).then(function(data) {
        return data;
      })
    });
  },
  setupController: function(controller, models) {
    controller.set('content', models.dc);
    controller.set('nodes', models.nodes);
    controller.set('dcs', models.dcs);
    controller.set('coordinates', models.coordinates);
    controller.set('isDropdownVisible', false);
  }
});
