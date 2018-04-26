import Route from '@ember/routing/route';
import { inject as service } from '@ember/service';
import { hash } from 'rsvp';
import { get } from '@ember/object';

export default Route.extend({
  repo: service('services'),
  model: function(params) {
    const repo = get(this, 'repo');
    return hash({
      model: repo.findBySlug(params.name, this.modelFor('dc').dc.Name),
    }).then(function(model) {
      // TODO: isolate, quick read of this some sort of filter might fit here instead of reduce?
      // come back and check exactly what this is doing and test
      return {
        ...model,
        ...{
          items: model.model,
          tags: model.model
            .reduce(function(prev, item) {
              return item.Service.Tags !== null ? prev.concat(item.Service.Tags) : prev;
            }, [])
            .filter(function(n) {
              return n !== undefined;
            })
            .uniq()
            .join(', '),
        },
      };
    });
  },
  setupController: function(controller, model) {
    this._super(...arguments);
    controller.setProperties(model);
  },
});
