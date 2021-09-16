import Route from 'consul-ui/routing/route';

export default class InstancesRoute extends Route {
  queryParams = {
    sortBy: 'sort',
    status: 'status',
    source: 'source',
    searchproperty: {
      as: 'searchproperty',
      empty: [['Name', 'Tags', 'ID', 'Address', 'Port', 'Service.Meta', 'Node.Meta']],
    },
    search: {
      as: 'filter',
      replace: true,
    },
  };
}
