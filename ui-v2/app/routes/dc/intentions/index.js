import Route from '@ember/routing/route';

export default Route.extend({
  queryParams: {
    filterBy: {
      as: 'action',
    },
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
    controller.setProperties(model);
  },
});
