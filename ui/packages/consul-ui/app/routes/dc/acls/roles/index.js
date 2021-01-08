import { inject as service } from '@ember/service';
import Route from 'consul-ui/routing/route';
import { hash } from 'rsvp';

import WithRoleActions from 'consul-ui/mixins/role/with-actions';

export default class IndexRoute extends Route.extend(WithRoleActions) {
  @service('repository/role') repo;

  queryParams = {
    sortBy: 'sort',
    searchproperty: {
      as: 'searchproperty',
      empty: [['Name', 'Description', 'Policy']],
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
      searchProperties: this.queryParams.searchproperty.empty[0],
    });
  }

  setupController(controller, model) {
    super.setupController(...arguments);
    controller.setProperties(model);
  }
}
