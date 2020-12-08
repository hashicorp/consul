import { inject as service } from '@ember/service';
import Route from 'consul-ui/routing/route';
import { hash } from 'rsvp';
import { get } from '@ember/object';

import WithAclActions from 'consul-ui/mixins/acl/with-actions';

export default class IndexRoute extends Route.extend(WithAclActions) {
  @service('repository/acl') repo;

  @service('settings') settings;

  queryParams = {
    sortBy: 'sort',
    kind: 'kind',
    search: {
      as: 'filter',
      replace: true,
    },
  };

  beforeModel(transition) {
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
  }

  model(params) {
    return hash({
      items: this.repo.findAllByDatacenter(this.modelFor('dc').dc.Name),
      token: this.settings.findBySlug('token'),
    });
  }

  setupController(controller, model) {
    super.setupController(...arguments);
    controller.setProperties(model);
  }
}
