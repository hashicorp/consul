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
      item: repo.findInstanceBySlug(params.id, params.name, dc),
    }).then(function(model) {
      return hash({
        proxy:
          get(service, 'Kind') !== 'connect-proxy'
            ? proxyRepo.findInstanceBySlug(params.id, params.name, dc)
            : null,
        ...model,
      });
    });
  },
  setupController: function(controller, model) {
    this._super(...arguments);
    controller.setProperties(model);
  },
});
