import { inject as service } from '@ember/service';
import Route from 'consul-ui/routing/route';
import { hash } from 'rsvp';

export default class IndexRoute extends Route {
  @service('repository/auth-method') repo;

  queryParams = {
    sortBy: 'sort',
    source: 'source',
    kind: 'kind',
    searchproperty: {
      as: 'searchproperty',
      empty: [['Name', 'DisplayName']],
    },
    search: {
      as: 'filter',
      replace: true,
    },
  };

  model(params) {
    return hash({
      ...this.repo.status({
        items: this.repo.findAllByDatacenter({
          dc: this.modelFor('dc').dc.Name,
          ns: this.optionalParams().nspace,
        }),
      }),
      searchProperties: this.queryParams.searchproperty.empty[0],
    });
  }

  setupController(controller, model) {
    super.setupController(...arguments);
    controller.setProperties(model);
  }
}
