import Route from '@ember/routing/route';

import { inject as service } from '@ember/service';
import ascend from 'consul-ui/utils/ascend';
import { hash } from 'rsvp';

export default Route.extend({
  repo: service('kv'),
  // sessionRepo: service('session'),
  model: function(params) {
    const key = params.key || '/';
    const parentKey = ascend(key, 1) || '/';
    const dc = this.modelFor('dc').dc;
    const repo = this.get('repo');
    return hash({
      isLoading: false,
      // better name, slug vs key?
      items: repo.findAllBySlug(parentKey, dc),
      parentKey: parentKey,
      grandParentKey: ascend(key, 2),
    });
  },
  setupController: function(controller, model) {
    controller.setProperties(model);
  },
});
