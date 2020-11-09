import Route from '@ember/routing/route';

export default class TopologyRoute extends Route {
  model() {
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
