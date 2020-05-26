import Route from '@ember/routing/route';
import { inject as service } from '@ember/service';
import { hash } from 'rsvp';
import { get } from '@ember/object';

export default Route.extend({
  repo: service('repository/service'),
  intentionRepo: service('repository/intention'),
  chainRepo: service('repository/discovery-chain'),
  proxyRepo: service('repository/proxy'),
  settings: service('settings'),
  model: function(params, transition = {}) {
    const dc = this.modelFor('dc').dc.Name;
    const nspace = this.modelFor('nspace').nspace.substr(1);
    return hash({
      item: this.repo.findBySlug(params.name, dc, nspace),
      urls: this.settings.findBySlug('urls'),
      dc: dc,
      nspace: nspace || 'default',
      proxies: [],
    })
      .then(model => {
        return ['connect-proxy', 'mesh-gateway', 'ingress-gateway', 'terminating-gateway'].includes(
          get(model, 'item.Service.Kind')
        )
          ? model
          : hash({
              intentions: this.intentionRepo
                .findByService(params.name, dc, nspace)
                .catch(function() {
                  return null;
                }),
              chain: this.chainRepo.findBySlug(params.name, dc, nspace).catch(function(e) {
                const code = get(e, 'errors.firstObject.status');
                // Currently we are specifically catching a 500, but we return null
                // by default, so null for all errors.
                // The extra code here is mainly for documentation purposes
                // and for if we need to perform different actions based on the error code
                // in the future
                switch (code) {
                  case '500':
                    // connect is likely to be disabled
                    // we just return a null to hide the tab
                    // `Connect must be enabled in order to use this endpoint`
                    return null;
                  default:
                    return null;
                }
              }),
              proxies: this.proxyRepo.findAllBySlug(params.name, dc, nspace),
              ...model,
            });
      })
      .then(model => {
        return ['ingress-gateway', 'terminating-gateway'].includes(get(model, 'item.Service.Kind'))
          ? hash({
              gateway: this.repo.findGatewayBySlug(params.name, dc, nspace),
              ...model,
            })
          : model;
      });
  },
  setupController: function(controller, model) {
    controller.setProperties(model);
  },
});
