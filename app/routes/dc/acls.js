import Route from '@ember/routing/route';

import Kv from 'consul-ui/models/dc/acl';
import get from 'consul-ui/lib/request/get';
export default Route.extend({
  model: function(params) {
    var dc = this.modelFor('dc').dc;
    // Return a promise containing the ACLS
    return get('/v1/acl/list', dc).then(function(data) {
      var objs = [];
      data.map(function(obj){
        if (obj.ID === "anonymous") {
          objs.unshift(Acl.create(obj));
        } else {
          objs.push(Acl.create(obj));
        }
      });
      return objs;
    });
  },
  actions: {
    error: function(error, transition) {
      // If consul returns 401, ACLs are disabled
      if (error && error.status === 401) {
        this.transitionTo('dc.aclsdisabled');
        // If consul returns 403, they key isn't authorized for that
        // action.
      } else if (error && error.status === 403) {
        this.transitionTo('dc.unauthorized');
      }
      return true;
    }
  },
  setupController: function(controller, model) {
    controller.set('acls', model);
    controller.set('newAcl', Acl.create());
  }

});
