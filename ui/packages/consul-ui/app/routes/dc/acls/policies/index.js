import { inject as service } from '@ember/service';
import Route from 'consul-ui/routing/route';
import { hash } from 'rsvp';

import WithPolicyActions from 'consul-ui/mixins/policy/with-actions';

export default class IndexRoute extends Route.extend(WithPolicyActions) {
  @service('repository/policy') repo;

  queryParams = {
    sortBy: 'sort',
    dc: 'dc',
    kind: 'kind',
    searchproperty: {
      as: 'searchproperty',
      empty: [['Name', 'Description']],
    },
    search: {
      as: 'filter',
      replace: true,
    },
  };

  model(params) {
    return hash({
      ...this.repo.status({
        items: this.repo.findAllByDatacenter(
          this.modelFor('dc').dc.Name,
          this.modelFor('nspace').nspace.substr(1)
        ),
      }),
    });
  }

  setupController(controller, model) {
    super.setupController(...arguments);
    controller.setProperties(model);
  }
}
