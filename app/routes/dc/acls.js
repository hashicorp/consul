import Route from '@ember/routing/route';
import { inject as service } from '@ember/service';
import { hash } from 'rsvp';

import { next } from '@ember/runloop';
import WithFeedback from 'consul-ui/mixins/with-feedback';
export default Route.extend(WithFeedback, {
  repo: service('acls'),
  model: function(params) {
    const repo = this.get('repo');
    const dc = this.modelFor('dc').dc;
    const newItem = repo.create();
    newItem.set('Datacenter', dc);
    return hash({
      items: repo.findAllByDatacenter(dc),
      newAcl: newItem,
      isShowingItem: false,
    });
  },
  setupController: function(controller, model) {
    controller.setProperties(model);
  },
  actions: {
    // temporary, better than previous
    // TODO: look at nodes/services for responsive stuff
    didTransition: function() {
      next(() => {
        // TODO: hasOutlet
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
