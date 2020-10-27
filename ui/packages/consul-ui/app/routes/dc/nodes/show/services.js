import Route from 'consul-ui/routing/route';

export default Route.extend({
  queryParams: {
    search: {
      as: 'filter',
      replace: true,
    },
  },
  model: function() {
    const parent = this.routeName
      .split('.')
      .slice(0, -1)
      .join('.');
    return this.modelFor(parent);
  },
  setupController: function(controller, model) {
    this._super(...arguments);
    controller.setProperties(model);
  },
});
