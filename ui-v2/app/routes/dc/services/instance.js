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
        proxy:
          // proxies and mesh-gateways can't have proxies themselves so don't even look
          ['connect-proxy', 'mesh-gateway'].includes(get(model.item, 'Kind'))
            ? null
            : this.proxyRepo.findInstanceBySlug(params.id, params.node, params.name, dc, nspace),
        ...model,
      });
    });
  },
  setupController: function(controller, model) {
    controller.setProperties(model);
  },
});
