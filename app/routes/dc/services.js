import Route from '@ember/routing/route';

import get from 'consul-ui/lib/request/get';
import map from 'consul-ui/lib/map';
import Service from 'consul-ui/models/dc/service';

export default Route.extend({
  model: function(params) {
    var dc = this.modelFor('dc').dc;
    // Return a promise to retrieve all of the services
    return get('/v1/internal/ui/services', dc).then(map(Service));
  },
  setupController: function(controller, model) {
    controller.set('services', model);
  }
});
