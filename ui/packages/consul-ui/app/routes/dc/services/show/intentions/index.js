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

  model(params) {
    return {
      dc: this.modelFor('dc').dc.Name,
      nspace: this.modelFor('nspace').nspace.substr(1) || 'default',
      slug: this.paramsFor('dc.services.show').name,
    };
  }

  setupController(controller, model) {
    super.setupController(...arguments);
    controller.setProperties(model);
  }
}
