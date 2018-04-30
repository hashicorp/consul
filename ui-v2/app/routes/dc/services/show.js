import Route from '@ember/routing/route';
import { inject as service } from '@ember/service';
import { hash } from 'rsvp';
import { get } from '@ember/object';

export default Route.extend({
  repo: service('services'),
  model: function(params) {
    const repo = get(this, 'repo');
    return hash({
      item: repo.findBySlug(params.name, this.modelFor('dc').dc.Name),
    }).then(function(model) {
      // TODO: isolate, quick read of this some sort of filter might fit here instead of reduce?
      // come back and check exactly what this is doing and test
      return {
        ...model,
        ...{
          items: model.item.Nodes,
        },
      };
    });
  },
  setupController: function(controller, model) {
    this._super(...arguments);
    controller.setProperties(model);
  },
});
