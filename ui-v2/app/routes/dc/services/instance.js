import Route from '@ember/routing/route';
import { inject as service } from '@ember/service';
import { hash } from 'rsvp';
import { get } from '@ember/object';

export default Route.extend({
  data: service('data-source/service'),
  model: function(params) {
    const dc = this.modelFor('dc').dc.Name;
    const nspace = this.modelFor('nspace').nspace.substr(1) || 'default';
    return hash({
      dc: dc,
      nspace: nspace,
      item: this.data.source(
        uri => uri`/${nspace}/${dc}/service-instance/${params.id}/${params.node}/${params.name}`
      ),
    }).then(model => {
      // this will not be run in a blocking loop, but this is ok as
      // its highly unlikely that a service will suddenly change to being a
      // connect-proxy or vice versa so leave as is for now
      return hash({
        ...model,
        proxyMeta:
          // proxies and mesh-gateways can't have proxies themselves so don't even look
          ['connect-proxy', 'mesh-gateway'].includes(get(model.item, 'Kind'))
            ? null
            : this.data.source(
                uri =>
                  uri`/${nspace}/${dc}/proxy-instance/${params.id}/${params.node}/${params.name}`
              ),
      }).then(model => {
        if (typeof get(model, 'proxyMeta.ServiceID') === 'undefined') {
          return model;
        }
        const proxy = {
          id: get(model, 'proxyMeta.ServiceID'),
          node: get(model, 'proxyMeta.Node'),
          name: get(model, 'proxyMeta.ServiceName'),
        };
        return hash({
          ...model,
          // Proxies have identical dc/nspace as their parent instance
          // No need to use Proxy's dc/nspace response
          proxy: this.data.source(
            uri => uri`/${nspace}/${dc}/service-instance/${proxy.id}/${proxy.node}/${proxy.name}`
          ),
        });
      });
    });
  },
  setupController: function(controller, model) {
    controller.setProperties(model);
  },
});
