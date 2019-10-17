import Route from '@ember/routing/route';
import { inject as service } from '@ember/service';
import { hash } from 'rsvp';

import WithNspaceActions from 'consul-ui/mixins/nspace/with-actions';

export default Route.extend(WithNspaceActions, {
  repo: service('repository/nspace'),
  isCreate: function(params, transition) {
    return transition.targetName.split('.').pop() === 'create';
  },
  model: function(params, transition) {
    const repo = this.repo;
    const create = this.isCreate(...arguments);
    const dc = this.modelFor('dc').dc.Name;
    return hash({
      isLoading: false,
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
        : repo.findBySlug(params.name),
    });
  },
  setupController: function(controller, model) {
    controller.setProperties(model);
  },
});
