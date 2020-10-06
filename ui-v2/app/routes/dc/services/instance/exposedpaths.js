import Route from 'consul-ui/routing/route';
import { get } from '@ember/object';

export default Route.extend({
  model: function() {
    const parent = this.routeName
      .split('.')
      .slice(0, -1)
      .join('.');
    return this.modelFor(parent);
  },
  afterModel: function(model, transition) {
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
  },
  setupController: function(controller, model) {
    this._super(...arguments);
    controller.setProperties(model);
  },
});
