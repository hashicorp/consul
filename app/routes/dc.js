import Route from '@ember/routing/route';
import { inject as service } from '@ember/service';
import { hash } from 'rsvp';

export default Route.extend({
  repo: service('dc'),
  // nodeRepo: service('nodes'),
  model: function(params) {
    const repo = this.get('repo');
    // const nodeRepo = this.get('nodeRepo');
    const dc = { Name: params.dc }; // TODO: this needs to be an ember-data object
    return hash({
      dc: dc,
      dcs: repo.findAll(),
      // TODO: Nodes should already be loaded on the selected
      // dc, we only need them for the selected dc
      // nodes: nodeRepo.findAllByDatacenter(dc.Name),
    });
  },
  setupController: function(controller, model) {
    controller.setProperties(model);
  },
});
