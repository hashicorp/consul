import Route from '@ember/routing/route';
import { inject as service } from '@ember/service';
import { hash } from 'rsvp';
import { assign } from '@ember/polyfills';

export default Route.extend({
  repo: service('nodes'),
  model: function(params) {
    const repo = this.get('repo');
    return hash({
      items: repo.findAllByDatacenter(this.modelFor('dc').dc),
    }).then(function(hash) {
      return assign(
        {},
        {
          unhealthy: hash.items.filter(item => item.get('hasFailingChecks')),
          healthy: hash.items.filter(item => !item.get('hasFailingChecks')),
        }
      );
    });
  },
  setupController: function(controller, model) {
    console.log(model);
    controller.setProperties(model);
  },
});
