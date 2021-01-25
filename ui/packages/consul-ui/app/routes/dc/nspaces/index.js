import { inject as service } from '@ember/service';
import Route from 'consul-ui/routing/route';

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

  async model(params) {
    return {
      items: await this.data.source(uri => uri`/*/*/namespaces`),
      searchProperties: this.queryParams.searchproperty.empty[0],
    };
  }

  setupController(controller, model) {
    super.setupController(...arguments);
    controller.setProperties(model);
  }
}
