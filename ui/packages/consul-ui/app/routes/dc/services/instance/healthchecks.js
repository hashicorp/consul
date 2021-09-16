import Route from 'consul-ui/routing/route';

export default class HealthchecksRoute extends Route {
  queryParams = {
    sortBy: 'sort',
    status: 'status',
    check: 'check',
    searchproperty: {
      as: 'searchproperty',
      empty: [['Name', 'Node', 'CheckID', 'Notes', 'Output', 'ServiceTags']],
    },
    search: {
      as: 'filter',
      replace: true,
    },
  };
}
