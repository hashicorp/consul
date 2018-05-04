import Route from '@ember/routing/route';
import { inject as service } from '@ember/service';
import { hash } from 'rsvp';
import { get } from '@ember/object';

export default Route.extend({
  repo: service('services'),
  queryParams: {
    s: {
      as: 'filter',
      replace: true,
    },
  },
  model: function(params) {
    const repo = get(this, 'repo');
    return hash({
      item: repo.findBySlug(params.name, this.modelFor('dc').dc.Name),
    }).then(function(model) {
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
