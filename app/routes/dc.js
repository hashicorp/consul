import Route from '@ember/routing/route';
import { inject as service } from '@ember/service';
import { hash } from 'rsvp';
import convertToLegacyDc from 'consul-ui/utils/convertToLegacyDc';

export default Route.extend({
  repo: service('dc'),
  nodeRepo: service('nodes'),
  model: function(params) {
    const repo = this.get('repo');
    const nodeRepo = this.get('nodeRepo');
    return hash({
      dc: params.dc, // TODO: this needs to be an ember-data object
      dcs: repo.findAll(),
      // TODO: Nodes should already be loaded on the selected
      // dc, we only need them for the selected dc
      nodes: nodeRepo.findAllByDatacenter(params.dc),
    }).then(
      // temporarily turn back into a pojo so
      // I don't have to touch the view
      convertToLegacyDc('dcs')
    );
  },
  setupController: function(controller, model) {
    controller.setProperties(model);
  },
});
