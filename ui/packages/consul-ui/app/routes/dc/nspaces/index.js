import { inject as service } from '@ember/service';
import Route from 'consul-ui/routing/route';

import WithBlockingActions from 'consul-ui/mixins/with-blocking-actions';
export default class IndexRoute extends Route.extend(WithBlockingActions) {
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
}
