import Route from 'consul-ui/routing/route';

export default Route.extend({
  model: function(params, transition) {
    return {
      nspace: '*',
      dc: this.paramsFor('dc').dc,
      service: this.paramsFor('dc.services.show').name,
      src: params.intention_id,
    };
  },
  setupController: function(controller, model) {
    this._super(...arguments);
    controller.setProperties(model);
  },
});
