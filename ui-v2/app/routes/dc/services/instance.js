import Route from '@ember/routing/route';
import { inject as service } from '@ember/service';
import { hash } from 'rsvp';
import { get } from '@ember/object';

export default Route.extend({
  repo: service('repository/service'),
  proxyRepo: service('repository/proxy'),
  model: function(params) {
    const dc = this.modelFor('dc').dc.Name;
    const nspace = this.modelFor('nspace').nspace.substr(1);
    return hash({
      item: this.repo.findInstanceBySlug(params.id, params.node, params.name, dc, nspace),
    }).then(model => {
      // this will not be run in a blocking loop, but this is ok as
      // its highly unlikely that a service will suddenly change to being a
      // connect-proxy or vice versa so leave as is for now
      return hash({
        proxyMeta:
          // proxies and mesh-gateways can't have proxies themselves so don't even look
          ['connect-proxy'].includes(get(model.item, 'Kind'))
            ? null
            : this.proxyRepo.findInstanceBySlug(params.id, params.node, params.name, dc, nspace),
        ...model,
      }).then(model => {
        if (get(model, 'proxyMeta.ServiceID') === undefined) {
          return model;
        }
        const proxyName = get(model, 'proxyMeta.ServiceName');
        const proxyID = get(model, 'proxyMeta.ServiceID');
        const proxyNode = get(model, 'proxyMeta.Node');
        return hash({
          // Proxies have identical dc/nspace as their parent instance
          // No need to use Proxy's dc/nspace response
          proxy: this.repo.findInstanceBySlug(proxyID, proxyNode, proxyName, dc, nspace),
          ...model,
        });
      });
    });
  },
  setupController: function(controller, model) {
    controller.setProperties(model);
  },
});
