import Route from '@ember/routing/route';
import { inject as service } from '@ember/service';
import { hash } from 'rsvp';
import { get } from '@ember/object';

export default Route.extend({
  repo: service('repository/service'),
  proxyRepo: service('repository/proxy'),
  model: function(params) {
    const repo = get(this, 'repo');
    const proxyRepo = get(this, 'proxyRepo');
    const dc = this.modelFor('dc').dc.Name;
    return hash({
      item: repo.findInstanceBySlug(params.id, params.node, params.name, dc),
    }).then(function(model) {
      // this will not be run in a blocking loop, but this is ok as
      // its highly unlikely that a service will suddenly change to being a
      // connect-proxy or vice versa so leave as is for now
      return hash({
        proxy:
          get(model.item, 'Kind') === 'connect-proxy'
            ? null
            : proxyRepo.findInstanceBySlug(params.id, params.node, params.name, dc),
        ...model,
      });
    });
  },
  setupController: function(controller, model) {
    controller.setProperties(model);
  },
});
