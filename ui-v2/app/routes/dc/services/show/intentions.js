import Route from '@ember/routing/route';

export default Route.extend({
  model: function() {
    return {
      routeName: this.routeName,
    };
  },
  setupController: function(controller, model) {
    controller.setProperties(model);
  },
});
