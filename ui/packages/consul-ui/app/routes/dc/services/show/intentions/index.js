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

  async model(params) {
    return {
      dc: this.modelFor('dc').dc.Name,
      nspace: this.modelFor('nspace').nspace.substr(1) || 'default',
      slug: this.paramsFor('dc.services.show').name,
      searchProperties: this.queryParams.searchproperty.empty[0],
    };
  }

  setupController(controller, model) {
    super.setupController(...arguments);
    controller.setProperties(model);
  }
}
