import Route from 'consul-ui/routing/route';

export default class InstancesRoute extends Route {
  queryParams = {
    sortBy: 'sort',
    status: 'status',
    source: 'source',
    searchproperty: {
      as: 'searchproperty',
      empty: [['Name', 'Tags', 'ID', 'Port']],
    },
    search: {
      as: 'filter',
      replace: true,
    },
  };

  model() {
    const parent = this.routeName
      .split('.')
      .slice(0, -1)
      .join('.');
    return this.modelFor(parent);
  }

  setupController(controller, model) {
    super.setupController(...arguments);
    controller.setProperties(model);
  }
}
