import Route from '@ember/routing/route';
import { inject as service } from '@ember/service';
import { hash } from 'rsvp';
import { get } from '@ember/object';

export default Route.extend({
  data: service('data-source/service'),
  settings: service('settings'),
  model: function(params, transition = {}) {
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
      proxies: [],
    })
      .then(model => {
        return ['connect-proxy', 'mesh-gateway', 'ingress-gateway', 'terminating-gateway'].includes(
          get(model, 'items.firstObject.Service.Kind')
        )
          ? model
          : hash({
              ...model,
              chain: this.data.source(uri => uri`/${nspace}/${dc}/discovery-chain/${params.name}`),
              proxies: this.data.source(
                uri => uri`/${nspace}/${dc}/proxies/for-service/${params.name}`
              ),
            });
      })
      .then(model => {
        return ['ingress-gateway', 'terminating-gateway'].includes(
          get(model, 'items.firstObject.Service.Kind')
        )
          ? hash({
              ...model,
              gatewayServices: this.data.source(
                uri => uri`/${nspace}/${dc}/gateways/for-service/${params.name}`
              ),
            })
          : model;
      });
  },
  setupController: function(controller, model) {
    controller.setProperties(model);
  },
});
