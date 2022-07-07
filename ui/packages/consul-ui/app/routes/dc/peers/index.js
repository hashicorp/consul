import Route from 'consul-ui/routing/route';

export default class PeersRoute extends Route {
  queryParams = {
    sortBy: 'sort',
    state: 'state',
    searchproperty: {
      as: 'searchproperty',
      empty: [['Name']],
    },
    search: {
      as: 'filter',
      replace: true,
    },
  };
}
