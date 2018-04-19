import Route from '@ember/routing/route';
import { inject as service } from '@ember/service';
import { hash } from 'rsvp';
import { get } from '@ember/object';

export default Route.extend({
  repo: service('acls'),
  model: function(params) {
    return hash({
      items: get(this, 'repo').findAllByDatacenter(this.modelFor('dc').dc.Name),
    });
  },
  setupController: function(controller, model) {
    controller.setProperties(model);
  },
  actions: {
    // TODO: this needs to happen for all endpoints
    error: function(e, transition) {
      if (e.errors[0].status === '401') {
        // 401 - ACLs are disabled
        this.transitionTo('dc.aclsdisabled');
        return false;
      } else if (e.errors[0].status === '403') {
        // 403 - the key isn't authorized for that action.
        this.transitionTo('dc.unauthorized');
        return false;
      }
      return true; // ??
    },
  },
});
