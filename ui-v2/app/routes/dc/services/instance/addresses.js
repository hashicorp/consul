import Route from '@ember/routing/route';
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
    if (get(model, 'item.Service.Kind') !== 'mesh-gateway') {
      const parent = this.routeName
        .split('.')
        .slice(0, -1)
        .join('.');
      this.replaceWith(parent);
    }
  },
  setupController: function(controller, model) {
    controller.setProperties(model);
  },
});
