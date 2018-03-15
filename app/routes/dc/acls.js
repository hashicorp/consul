import Route from '@ember/routing/route';
import { inject as service } from '@ember/service';
import { hash } from 'rsvp';

import { next } from '@ember/runloop';
export default Route.extend({
  repo: service('acls'),
  model: function(params) {
    const repo = this.get('repo');
    return hash({
      items: repo.findAllByDatacenter(this.modelFor('dc').dc),
      newAcl: repo.create(),
      isShowingItem: false,
    });
  },
  setupController: function(controller, model) {
    controller.setProperties(model);
  },
  actions: {
    // temporary, better than previous
    didTransition: function() {
      next(() => {
        this.controller.setProperties({
          isShowingItem: this.get('router.currentPath') === 'dc.acls.show',
        });
      });
    },
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
    create: function(acl) {
      this.get('feedback').execute(
        () => {
          return acl.save().then(() => {
            this.refresh();
          });
        },
        `Created ${acl.get('Name')}`,
        `There was an error using ${acl.get('Name')}`
      );
    },
  },
});
