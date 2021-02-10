import { inject as service } from '@ember/service';
import Route from 'consul-ui/routing/route';
import { hash } from 'rsvp';

import WithNspaceActions from 'consul-ui/mixins/nspace/with-actions';

export default class EditRoute extends Route.extend(WithNspaceActions) {
  @service('repository/nspace')
  repo;

  isCreate(params, transition) {
    return transition.targetName.split('.').pop() === 'create';
  }

  model(params, transition) {
    const repo = this.repo;
    const create = this.isCreate(...arguments);
    const dc = this.modelFor('dc').dc.Name;
    return hash({
      create: create,
      dc: dc,
      item: create
        ? Promise.resolve(
            repo.create({
              ACLs: {
                PolicyDefaults: [],
                RoleDefaults: [],
              },
            })
          )
        : repo.findBySlug({ id: params.name }),
    });
  }

  setupController(controller, model) {
    super.setupController(...arguments);
    controller.setProperties(model);
  }
}
