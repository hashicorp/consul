import Route from '@ember/routing/route';
import { get } from '@ember/object';

export default Route.extend({
  afterModel: function(model, transition) {
    const parent = this.routeName
      .split('.')
      .slice(0, -1)
      .join('.');
    // the default selected tab depends on whether you have any healthchecks or not
    // so check the length here.
    let to = 'topology';
    const parentModel = this.modelFor(parent);

    const kind = get(parentModel, 'items.firstObject.Service.Kind');

    switch (kind) {
      case 'ingress-gateway':
        if (!get(parentModel, 'topology.Exists')) {
          to = 'upstreams';
        }
        break;
      case 'terminating-gateway':
        to = 'services';
        break;
      case 'mesh-gateway':
        to = 'instances';
        break;
      default:
        if (!get(parentModel, 'topology.Exists')) {
          to = 'instances';
        }
    }

    this.replaceWith(`${parent}.${to}`, parentModel);
  },
});
