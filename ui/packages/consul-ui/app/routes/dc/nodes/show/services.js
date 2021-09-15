import Route from 'consul-ui/routing/route';

export default class ServicesRoute extends Route {
  queryParams = {
    sortBy: 'sort',
    status: 'status',
    source: 'source',
    searchproperty: {
      as: 'searchproperty',
      empty: [['Name', 'Tags', 'ID', 'Address', 'Port', 'Service.Meta']],
    },
    search: {
      as: 'filter',
      replace: true,
    },
  };
}
