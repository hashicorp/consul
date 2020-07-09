import Route from '@ember/routing/route';
import { inject as service } from '@ember/service';
import { hash } from 'rsvp';
import { get } from '@ember/object';

export default Route.extend({
  repo: service('repository/service'),
  chainRepo: service('repository/discovery-chain'),
  proxyRepo: service('repository/proxy'),
  settings: service('settings'),
  model: function(params, transition = {}) {
    const dc = this.modelFor('dc').dc.Name;
    const nspace = this.modelFor('nspace').nspace.substr(1);
    return hash({
      slug: params.name,
      dc: dc,
      nspace: nspace || 'default',
      item: this.repo.findBySlug(params.name, dc, nspace),
      urls: this.settings.findBySlug('urls'),
      proxies: [],
    })
      .then(model => {
        return ['connect-proxy', 'mesh-gateway', 'ingress-gateway', 'terminating-gateway'].includes(
          get(model, 'item.Service.Kind')
        )
          ? model
          : hash({
              chain: this.chainRepo.findBySlug(params.name, dc, nspace),
              proxies: this.proxyRepo.findAllBySlug(params.name, dc, nspace),
              ...model,
            });
      })
      .then(model => {
        return ['ingress-gateway', 'terminating-gateway'].includes(get(model, 'item.Service.Kind'))
          ? hash({
              gatewayServices: this.repo.findGatewayBySlug(params.name, dc, nspace),
              ...model,
            })
          : model;
      });
  },
  setupController: function(controller, model) {
    controller.setProperties(model);
  },
});
