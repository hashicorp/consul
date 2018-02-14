import Route from '@ember/routing/route';

import get from 'consul-ui/utils/request/get';
import map from 'consul-ui/utils/map';
import Node from 'consul-ui/models/dc/node';

export default Route.extend({
  model: function(params) {
    var dc = this.modelFor('dc').dc;
    // Here we just use the built-in health endpoint, as it gives us everything
    // we need.
    return get('/v1/health/service/' + params.name, dc).then(map(Node));
  },
  setupController: function(controller, model) {
    var tags = model
      .reduce(function(prev, item, i, arr) {
        return item.Service.Tags !== null ? prev.concat(item.Service.Tags) : prev;
      }, [])
      .filter(function(n) {
        return n !== undefined;
      })
      .uniq()
      .join(', ');
    controller.set('model', model);
    controller.set('tags', tags);
  },
});
