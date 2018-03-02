import Route from '@ember/routing/route';
import { inject as service } from '@ember/service';
import { hash } from 'rsvp';

export default Route.extend({
  repo: service('acls'),
  model: function(/* params */) {
    const repo = this.get('repo');
    return hash({
      acls: repo.findAllByDatacenter(this.modelFor('dc').dc),
      newAcl: repo.create(),
    });
  },
  setupController: function(controller, model) {
    controller.setProperties(model);
  },
  actions: {
    error: function(error) {
      // error, transition
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
});
