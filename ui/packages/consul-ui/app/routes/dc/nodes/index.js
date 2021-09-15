import Route from 'consul-ui/routing/route';

export default class IndexRoute extends Route {
  queryParams = {
    sortBy: 'sort',
    status: 'status',
    searchproperty: {
      as: 'searchproperty',
      empty: [['Node', 'Address', 'Meta']],
    },
    search: {
      as: 'filter',
      replace: true,
    },
  };
}
