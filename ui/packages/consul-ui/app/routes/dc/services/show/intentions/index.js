import Route from 'consul-ui/routing/route';

export default class IndexRoute extends Route {
  queryParams = {
    sortBy: 'sort',
    access: 'access',
    searchproperty: {
      as: 'searchproperty',
      empty: [['SourceName', 'DestinationName']],
    },
    search: {
      as: 'filter',
      replace: true,
    },
  };
}
