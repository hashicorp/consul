import Route from '@ember/routing/route';
import { inject as service } from '@ember/service';
import { hash } from 'rsvp';
import { get, set } from '@ember/object';

import WithAclActions from 'consul-ui/mixins/acl/with-actions';

export default Route.extend(WithAclActions, {
  templateName: 'dc/acls/edit',
  repo: service('acls'),
  model: function(params) {
    const item = get(this, 'repo').create();
    set(item, 'Datacenter', this.modelFor('dc').dc.Name);
    return hash({
      create: true,
      isLoading: false,
      item: item,
      types: ['management', 'client'],
    });
  },
  setupController: function(controller, model) {
    controller.setProperties(model);
  },
});
