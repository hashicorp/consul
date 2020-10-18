import Route from 'consul-ui/routing/route';
import { inject as service } from '@ember/service';
import { hash } from 'rsvp';

export default Route.extend({
  data: service('data-source/service'),
  model: function() {
    const dc = this.modelFor('dc').dc.Name;
    const nspace = this.modelFor('nspace').nspace.substr(1);
    const parent = this.routeName
      .split('.')
      .slice(0, -1)
      .join('.');
    const name = this.modelFor(parent).name;
    return hash({
      dc: dc,
      nspace: nspace,
      gatewayServices: this.data.source(uri => uri`/${nspace}/${dc}/gateways/for-service/${name}`),
    });
  },
  setupController: function(controller, model) {
    this._super(...arguments);
    controller.setProperties(model);
  },
});
