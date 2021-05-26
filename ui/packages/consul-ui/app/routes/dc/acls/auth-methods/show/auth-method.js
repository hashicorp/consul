import Route from 'consul-ui/routing/route';

export default class AuthMethodRoute extends Route {
  model(params) {
    const parent = this.routeName
      .split('.')
      .slice(0, -1)
      .join('.');
    return this.modelFor(parent);
  }

  setupController(controller, model) {
    super.setupController(...arguments);
    controller.setProperties(model);
  }
}
