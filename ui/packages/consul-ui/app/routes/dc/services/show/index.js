import Route from 'consul-ui/routing/route';
import { get } from '@ember/object';

export default class IndexRoute extends Route {
  async afterModel(model, transition) {
    const parent = this.routeName
      .split('.')
      .slice(0, -1)
      .join('.');
    let to = 'topology';
    const parentModel = this.modelFor(parent);
    const hasProxy = get(parentModel, 'proxies.length') !== 0;
    const item = get(parentModel, 'items.firstObject');
    const kind = get(item, 'Service.Kind');
    const hasTopology = get(parentModel, 'dc.MeshEnabled') && get(item, 'IsMeshOrigin');
    switch (kind) {
      case 'ingress-gateway':
        if (!hasTopology) {
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
        if (!hasProxy || !hasTopology) {
          to = 'instances';
        }
    }
    this.replaceWith(`${parent}.${to}`, parentModel);
  }
}
