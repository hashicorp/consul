import Route from '@ember/routing/route';
import { inject as service } from '@ember/service';

export default Route.extend({
  repo: service('services'),
  model: function(params) {
    return this.get('repo').findBySlug(params.name, this.modelFor('dc').dc);
  },
  setupController: function(controller, model) {
    const tags = model
      .reduce(function(prev, item) {
        return item.Service.Tags !== null ? prev.concat(item.Service.Tags) : prev;
      }, [])
      .filter(function(n) {
        return n !== undefined;
      })
      .uniq()
      .join(', ');
    controller.set('model', model);
    controller.set('tags', tags);
  },
});
