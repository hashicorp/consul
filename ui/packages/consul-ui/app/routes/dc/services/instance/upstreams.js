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

  model() {
    const parent = this.routeName
      .split('.')
      .slice(0, -1)
      .join('.');
    return {
      ...this.modelFor(parent),
      searchProperties: this.queryParams.searchproperty.empty[0],
    };
  }

  setupController(controller, model) {
    super.setupController(...arguments);
    controller.setProperties(model);
  }
}
