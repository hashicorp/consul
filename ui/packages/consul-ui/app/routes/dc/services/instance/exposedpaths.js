import Route from 'consul-ui/routing/route';

export default class ExposedpathsRoute extends Route {
  model() {
    const parent = this.routeName
      .split('.')
      .slice(0, -1)
      .join('.');
    return this.modelFor(parent);
  },
  setupController: function(controller, model) {
    this._super(...arguments);
    controller.setProperties(model);
  }
}
