import Route from 'consul-ui/routing/route';

export default Route.extend({
  queryParams: {
    sortBy: 'sort',
    search: {
      as: 'filter',
      replace: true,
    },
  },
  model: function(params) {
    return {
      dc: this.modelFor('dc').dc.Name,
      nspace: this.modelFor('nspace').nspace.substr(1) || 'default',
    };
  },
  setupController: function(controller, model) {
    this._super(...arguments);
    controller.setProperties(model);
  },
});
