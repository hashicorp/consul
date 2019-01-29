import Route from '@ember/routing/route';
import { inject as service } from '@ember/service';
import { hash } from 'rsvp';
import { get } from '@ember/object';

export default Route.extend({
  repo: service('repository/service'),
  model: function(params) {
    const repo = get(this, 'repo');
    // TODO: findInstanceBySlug
    return hash({
      item: repo.findBySlug(params.name, this.modelFor('dc').dc.Name),
    }).then(function(model) {
      const i = model.item.Nodes.findIndex(function(item) {
        return item.Service.ID === params.id;
      });
      // console.log(model.item);
      const service = model.item.Nodes[i].Service;
      service.Node = model.item.Nodes[i].Node;
      service.ServiceChecks = model.item.Nodes[i].Checks.filter(function(item) {
        return item.ServiceID != '';
      });
      service.NodeChecks = model.item.Nodes[i].Checks.filter(function(item) {
        return item.ServiceID == '';
      });
      return {
        ...model,
        ...{
          item: service,
        },
      };
    });
  },
  setupController: function(controller, model) {
    this._super(...arguments);
    controller.setProperties(model);
  },
});
