import Route from '@ember/routing/route';
import { inject as service } from '@ember/service';
import { hash } from 'rsvp';
import { assign } from '@ember/polyfills';

export default Route.extend({
  repo: service('services'),
  model: function(params) {
    const repo = this.get('repo');
    return hash({
      model: repo.findBySlug(params.name, this.modelFor('dc').dc),
    }).then(function(model) {
      console.log(model.model.get('Nodes'));
      // TODO: isolate, quick read of this some sort of filter might fit here instead of reduce?
      // come back and check exactly what this is doing and test
      return assign({}, model, { tags: model.model.tags });
    });
  },
  setupController: function(controller, model) {
    controller.setProperties(model);
  },
});
