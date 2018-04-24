import Route from '@ember/routing/route';
import { inject as service } from '@ember/service';
import { hash } from 'rsvp';
import { get } from '@ember/object';

import WithFeedback from 'consul-ui/mixins/with-feedback';

export default Route.extend(WithFeedback, {
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
    delete: function(item) {
      get(this, 'feedback').execute(
        () => {
          return get(this, 'repo')
            .remove(item)
            .then(() => {
              return this.refresh();
            });
        },
        `Your key was deleted.`,
        `There was an error deleting your token.`
      );
    },
    use: function(item) {
      get(this, 'feedback').execute(
        () => {
          get(this, 'settings')
            .persist({ token: get(item, 'ID') })
            .then(() => {
              this.transitionTo('dc.services');
            });
        },
        `Now using new ACL token`,
        `There was an error using that ACL token`
      );
    },
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
