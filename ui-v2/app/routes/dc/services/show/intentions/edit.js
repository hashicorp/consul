import Route from '@ember/routing/route';

export default Route.extend({
  model: function(params, transition) {
    return {
      nspace: '*',
      dc: this.paramsFor('dc').dc,
      service: this.paramsFor('dc.services.show').name,
      src: params.intention,
    };
  },
  setupController: function(controller, model) {
    controller.setProperties(model);
  },
});
