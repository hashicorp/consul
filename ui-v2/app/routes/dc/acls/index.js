import Route from 'consul-ui/routing/route';
import { inject as service } from '@ember/service';
import { hash } from 'rsvp';
import { get } from '@ember/object';

import WithAclActions from 'consul-ui/mixins/acl/with-actions';

export default Route.extend(WithAclActions, {
  repo: service('repository/acl'),
  settings: service('settings'),
  queryParams: {
    search: {
      as: 'filter',
      replace: true,
    },
  },
  beforeModel: function(transition) {
    return this.settings.findBySlug('token').then(token => {
      // If you don't have a token set or you have a
      // token set with AccessorID set to not null (new ACL mode)
      // then rewrite to the new acls
      if (!token || get(token, 'AccessorID') !== null) {
        // If you return here, you get a TransitionAborted error in the tests only
        // everything works fine either way checking things manually
        this.replaceWith('dc.acls.tokens');
      }
    });
  },
  model: function(params) {
    return hash({
      items: this.repo.findAllByDatacenter(this.modelFor('dc').dc.Name),
      token: this.settings.findBySlug('token'),
    });
  },
  setupController: function(controller, model) {
    this._super(...arguments);
    controller.setProperties(model);
  },
});
