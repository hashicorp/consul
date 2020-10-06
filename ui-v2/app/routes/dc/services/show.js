import Route from 'consul-ui/routing/route';
import { inject as service } from '@ember/service';
import { hash } from 'rsvp';
import { get } from '@ember/object';

export default Route.extend({
  data: service('data-source/service'),
  settings: service('settings'),
  model: function(params, transition) {
    const dc = this.modelFor('dc').dc.Name;
    const nspace = this.modelFor('nspace').nspace.substr(1);
    return hash({
      slug: params.name,
      dc: dc,
      nspace: nspace,
      items: this.data.source(
        uri => uri`/${nspace}/${dc}/service-instances/for-service/${params.name}`
      ),
      urls: this.settings.findBySlug('urls'),
      chain: null,
      proxies: [],
      topology: null,
    })
      .then(model => {
        return ['connect-proxy', 'mesh-gateway', 'ingress-gateway', 'terminating-gateway'].includes(
          get(model, 'items.firstObject.Service.Kind')
        )
          ? model
          : hash({
              ...model,
              chain: this.data.source(uri => uri`/${nspace}/${dc}/discovery-chain/${params.name}`),
              // Whilst `proxies` isn't used anywhere in the show templates
              // it provides a relationship of ProxyInstance on the ServiceInstance
              // which can respond at a completely different blocking rate to
              // the ServiceInstance itself
              proxies: this.data.source(
                uri => uri`/${nspace}/${dc}/proxies/for-service/${params.name}`
              ),
            });
      })
      .then(model => {
        return ['mesh-gateway', 'terminating-gateway'].includes(
          get(model, 'items.firstObject.Service.Kind')
        )
          ? model
          : hash({
              ...model,
              topology: this.data.source(
                uri => uri`/${nspace}/${dc}/topology/for-service/${params.name}`
              ),
            });
      });
  },
  setupController: function(controller, model) {
    this._super(...arguments);
    controller.setProperties(model);
  },
});
