import Route from '@ember/routing/route';
import { inject as service } from '@ember/service';

export default Route.extend({
  workflow: service('workflow'),
  repo: service('dc'),
  nodeRepo: service('nodes'),
  model: function(params) {
    const repo = this.get('repo');
    const nodeRepo = this.get('nodeRepo');
    return this.get('workflow').execute(() => {
      return {
        item: params.dc,
        items: repo.findAll(),
        nodes: nodeRepo.findAllByDatacenter(params.dc),
        coordinates: nodeRepo.findAllCoordinatesByDatacenter(params.dc),
      };
    });
  },
  setupController: function(controller, model) {
    controller.setProperties(model);
  },
});
