import Route from 'consul-ui/routing/route';

export default class UpstreamsRoute extends Route {
  queryParams = {
    sortBy: 'sort',
    search: {
      as: 'filter',
      replace: true,
    },
    searchproperty: {
      as: 'searchproperty',
      empty: [['DestinationName', 'LocalBindAddress', 'LocalBindPort']],
    },
  };
}
