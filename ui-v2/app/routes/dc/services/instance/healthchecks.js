import Route from '@ember/routing/route';

export default Route.extend({
  model: function() {
    const parent = this.routeName
      .split('.')
      .slice(0, -1)
      .join('.');
    return this.modelFor(parent);
  },
  setupController: function(controller, model) {
    controller.setProperties(model);
  },
});
