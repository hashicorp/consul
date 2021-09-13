import Route from 'consul-ui/routing/route';
import WithBlockingActions from 'consul-ui/mixins/with-blocking-actions';

export default class IndexRoute extends Route.extend(WithBlockingActions) {
  queryParams = {
    sortBy: 'sort',
    kind: 'kind',
    searchproperty: {
      as: 'searchproperty',
      empty: [['AccessorID', 'Description', 'Role', 'Policy']],
    },
    search: {
      as: 'filter',
      replace: true,
    },
  };
}
