import Route from '@ember/routing/route';
import { inject as service } from '@ember/service';

export default Route.extend({
  workflow: service('workflow'),
  repo: service('dc'),
  nodeRepo: service('nodes'),
  model: function(params) {
    // Return a promise hash to retreieve the
    // dcs and nodes used in the header
    const repo = this.get('repo');
    const nodeRepo = this.get('nodeRepo');
    return this.get('workflow').execute(() => {
      return {
        dc: params.dc,
        dcs: repo.findAll(),
        nodes: nodeRepo.findAllByDatacenter(params.dc),
        coordinates: nodeRepo.findAllCoordinatesByDatacenter(params.dc),
      };
    });
  },
  setupController: function(controller, model) {
    controller.setProperties({
      content: model.dc,
      nodes: model.nodes,
      dcs: model.dcs,
      coordinates: model.coordinates,
      isDropdownVisible: false,
    });
  },
});
