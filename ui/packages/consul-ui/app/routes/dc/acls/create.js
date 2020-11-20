import { inject as service } from '@ember/service';
import Route from 'consul-ui/routing/route';
import { hash } from 'rsvp';
import { get } from '@ember/object';

import WithAclActions from 'consul-ui/mixins/acl/with-actions';

export default class CreateRoute extends Route.extend(WithAclActions) {
  templateName = 'dc/acls/edit';

  @service('repository/acl')
  repo;

  beforeModel() {
    this.repo.invalidate();
  }

  model(params) {
    this.item = this.repo.create({
      Datacenter: this.modelFor('dc').dc.Name,
    });
    return hash({
      create: true,
      item: this.item,
      types: ['management', 'client'],
    });
  }

  setupController(controller, model) {
    super.setupController(...arguments);
    controller.setProperties(model);
  }

  deactivate() {
    if (get(this.item, 'isNew')) {
      this.item.destroyRecord();
    }
  }
}
