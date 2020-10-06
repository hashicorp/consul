import Route from 'consul-ui/routing/route';

export default Route.extend({
  setupController: function(controller, model) {
    this._super(...arguments);
    controller.setProperties(model);
  },
});
