import Route from '@ember/routing/route';
import { inject as service } from '@ember/service';

export default Route.extend({
  repo: service('acls'),
  model: function(params) {
    // Return a promise containing the ACLS
    return this.get('repo').findByDatacenter(this.modelFor('dc').dc);
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
    },
  },
  setupController: function(controller, model) {
    controller.set('acls', model);
    controller.set('newAcl', this.get('repo').create());
  },
});
