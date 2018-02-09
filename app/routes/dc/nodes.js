import Route from '@ember/routing/route';

import get from 'consul-ui/lib/request/get';
import map from 'consul-ui/lib/map';
import Node from 'consul-ui/models/dc/node';

export default Route.extend({
  model: function(params) {
    var dc = this.modelFor('dc').dc;
    // Return a promise containing the nodes
    return get('/v1/internal/ui/nodes', dc).then(map(Node));
  },
  setupController: function(controller, model) {
    controller.set('nodes', model);
  }
});
