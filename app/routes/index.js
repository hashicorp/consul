import Route from '@ember/routing/route';

import { inject as service } from '@ember/service';

export default Route.extend({
  repo: service('dc'),
  model: function() {
    //params
    return this.get('repo').findAll();
  },
  afterModel: function(model) {
    //model, transition
    // If we only have one datacenter, jump
    // straight to it and bypass the global
    // view
    if (model.get('length') === 1) {
      this.transitionTo('dc.services', model[0]);
    }
  },
});
