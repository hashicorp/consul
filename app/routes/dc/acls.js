import Route from '@ember/routing/route';
import { inject as controller } from '@ember/controller';
import { inject as service } from '@ember/service';
import { hash } from 'rsvp';

import handle from 'consul-ui/utils/handle';

import { next } from '@ember/runloop';

export default Route.extend({
  repo: service('acls'),
  model: function(params) {
    const repo = this.get('repo');
    return hash({
      items: repo.findAllByDatacenter(this.modelFor('dc').dc),
      newAcl: repo.create(),
      isShowingItem: false,
      types: ['client', 'management'],
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
    error: function(error, transition) {
      // error, transition
      // If consul returns 401, ACLs are disabled
      if (error && error.status === 401) {
        this.transitionTo('dc.aclsdisabled');
        return false;
      } else if (error && error.status === 403) {
        // If consul returns 403, they key isn't authorized for that
        // action.
        this.transitionTo('dc.unauthorized');
        return false;
      }
      return true; // ??
    },
    createAcl: function(newAcl) {
      handle.bind(this)(
        () => {
          return newAcl.save().then(() => {
            this.refresh();
          });
        },
        'created',
        'errored'
      );
    },
  },
});
