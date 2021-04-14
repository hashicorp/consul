import { inject as service } from '@ember/service';
import Route from 'consul-ui/routing/route';
import { get } from '@ember/object';

export default class InstanceRoute extends Route {
  @service('data-source/service') data;

  async model(params, transition) {
    const dc = this.modelFor('dc').dc.Name;
    const nspace = this.optionalParams().nspace;

    const item = await this.data.source(
      uri => uri`/${nspace}/${dc}/service-instance/${params.id}/${params.node}/${params.name}`
    );

    let proxyMeta, proxy;
    if (get(item, 'IsOrigin')) {
      proxyMeta = await this.data.source(
        uri => uri`/${nspace}/${dc}/proxy-instance/${params.id}/${params.node}/${params.name}`
      );
      if (typeof get(proxyMeta, 'ServiceID') !== 'undefined') {
        const proxyParams = {
          id: get(proxyMeta, 'ServiceID'),
          node: get(proxyMeta, 'NodeName'),
          name: get(proxyMeta, 'ServiceName'),
        };
        // Proxies have identical dc/nspace as their parent instance
        // so no need to use Proxy's dc/nspace response
        // the proxy itself is just a normal service model
        proxy = await this.data.source(
          uri =>
            uri`/${nspace}/${dc}/proxy-service-instance/${proxyParams.id}/${proxyParams.node}/${proxyParams.name}`
        );
      }
    }

    return {
      dc,
      nspace,
      item,
      proxyMeta,
      proxy,
    };
  }

  setupController(controller, model) {
    super.setupController(...arguments);
    controller.setProperties(model);
  }
}
