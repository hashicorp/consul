import { inject as service } from '@ember/service';
import Route from 'consul-ui/routing/route';
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

  async beforeModel(transition) {
    const token = await this.settings.findBySlug('token');
    // If you don't have a token set or you have a
    // token set with AccessorID set to not null (new ACL mode)
    // then rewrite to the new acls
    if (!token || get(token, 'AccessorID') !== null) {
      // If you return here, you get a TransitionAborted error in the tests only
      // everything works fine either way checking things manually
      this.replaceWith('dc.acls.tokens');
    }
  }

  async model(params) {
    const _items = this.repo.findAllByDatacenter(this.modelFor('dc').dc.Name);
    const _token = this.settings.findBySlug('token');
    return {
      items: await _items,
      token: await _token,
    };
  }

  setupController(controller, model) {
    super.setupController(...arguments);
    controller.setProperties(model);
  }
}
