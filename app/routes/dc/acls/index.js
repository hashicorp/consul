import Route from '@ember/routing/route';
import { inject as service } from '@ember/service';
import { hash } from 'rsvp';

export default Route.extend({
  repo: service('acls'),
  model: function(params) {
    return hash({
      items: this.get('repo').findAllByDatacenter(this.modelFor('dc').dc)
    });
  },
  setupController: function(controller, model) {
    controller.setProperties(model);
  },
  actions: {
    // didTransition: function() {
    //   next(() => {
    //     // TODO: hasOutlet
    //     this.controller.setProperties({
    //       isShowingItem: this.get('router.currentPath') === 'dc.acls.show',
    //     });
    //   });
    // },
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
    }
  },
});
