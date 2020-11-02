import Route from 'consul-ui/routing/route';
import { get } from '@ember/object';

export default class ExposedpathsRoute extends Route {
  model() {
    const parent = this.routeName
      .split('.')
      .slice(0, -1)
      .join('.');
    return this.modelFor(parent);
  }

  afterModel(model, transition) {
    if (
      get(model, 'item.Kind') !== 'connect-proxy' ||
      get(model, 'item.Proxy.Expose.Paths.length') < 1
    ) {
      const parent = this.routeName
        .split('.')
        .slice(0, -1)
        .join('.');
      this.replaceWith(parent);
    }
  }

  setupController(controller, model) {
    super.setupController(...arguments);
    controller.setProperties(model);
  }
}
