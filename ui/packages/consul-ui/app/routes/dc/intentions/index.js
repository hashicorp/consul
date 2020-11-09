import Route from 'consul-ui/routing/route';

export default class IndexRoute extends Route {
  queryParams = {
    sortBy: 'sort',
    search: {
      as: 'filter',
      replace: true,
    },
  };

  model(params) {
    return {
      dc: this.modelFor('dc').dc.Name,
      nspace: this.modelFor('nspace').nspace.substr(1) || 'default',
    };
  }

  setupController(controller, model) {
    super.setupController(...arguments);
    controller.setProperties(model);
  }
}
