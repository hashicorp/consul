import { inject as service } from '@ember/service';
import Route from 'consul-ui/routing/route';
import { hash } from 'rsvp';

import WithNspaceActions from 'consul-ui/mixins/nspace/with-actions';
export default class IndexRoute extends Route.extend(WithNspaceActions) {
  @service('data-source/service') data;
  @service('repository/nspace') repo;

  queryParams = {
    sortBy: 'sort',
    searchproperty: {
      as: 'searchproperty',
      empty: [['Name', 'Description', 'Role', 'Policy']],
    },
    search: {
      as: 'filter',
      replace: true,
    },
  };

  model(params) {
    return hash({
      items: this.data.source(uri => uri`/*/*/namespaces`),
    });
  }

  setupController(controller, model) {
    super.setupController(...arguments);
    controller.setProperties(model);
  }
}
