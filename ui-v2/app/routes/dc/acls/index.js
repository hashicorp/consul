import Route from '@ember/routing/route';
import { inject as service } from '@ember/service';
import { hash } from 'rsvp';
import { get } from '@ember/object';

import WithAclActions from 'consul-ui/mixins/acl/with-actions';

export default Route.extend(WithAclActions, {
  repo: service('repository/acl'),
  settings: service('settings'),
  queryParams: {
    s: {
      as: 'filter',
      replace: true,
    },
  },
  beforeModel: function(transition) {
    return get(this, 'settings')
      .findBySlug('token')
      .then(token => {
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
      isLoading: false,
      items: get(this, 'repo').findAllByDatacenter(this.modelFor('dc').dc.Name),
      token: get(this, 'settings').findBySlug('token'),
    });
  },
  setupController: function(controller, model) {
    controller.setProperties(model);
  },
});
