import Route from '@ember/routing/route';
import { inject as service } from '@ember/service';
import { hash } from 'rsvp';

export default Route.extend({
  repo: service('dc'),
  nodeRepo: service('nodes'),
  model: function(params) {
    const repo = this.get('repo');
    const nodeRepo = this.get('nodeRepo');
    return hash({
      dc: params.dc,
      dcs: repo.findAll(),
      nodes: nodeRepo.findAllByDatacenter(params.dc),
      coordinates: nodeRepo.findAllCoordinatesByDatacenter(params.dc),
    });
  },
  setupController: function(controller, model) {
    controller.setProperties(model);
  },
});
