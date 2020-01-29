import Route from '@ember/routing/route';

export default Route.extend({
  model: function(params) {
    return {
      dc: this.modelFor('dc').dc.Name,
      nspace: this.modelFor('nspace').nspace.substr(1),
      slug: [params.id, params.node, params.name].join('/'),
      item: undefined,
      proxy: undefined,
    };
  },
  setupController: function(controller, model) {
    controller.setProperties(model);
  },
});
