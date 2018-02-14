import Route from '@ember/routing/route';

import get from 'consul-ui/utils/request/get';

export default Route.extend({
  model: function(params) {
    return get('/v1/catalog/datacenters').then(function(data) {
      return data;
    });
  },
  afterModel: function(model, transition) {
    // If we only have one datacenter, jump
    // straight to it and bypass the global
    // view
    if (model.get('length') === 1) {
      this.transitionTo('dc.services', model[0]);
    }
  },
});
