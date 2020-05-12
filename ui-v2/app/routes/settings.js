import Route from '@ember/routing/route';
import { inject as service } from '@ember/service';
import { hash } from 'rsvp';
import { get, set } from '@ember/object';

export default Route.extend({
  client: service('client/http'),
  repo: service('settings'),
  dcRepo: service('repository/dc'),
  nspacesRepo: service('repository/nspace/disabled'),
  model: function(params) {
    const app = this.modelFor('application');
    return hash({
      item: this.repo.findAll(),
      dc: this.dcRepo.getActive(undefined, app.dcs),
      nspace: this.nspacesRepo.getActive(),
    }).then(model => {
      if (typeof get(model.item, 'client.blocking') === 'undefined') {
        set(model, 'item.client', { blocking: true });
      }
      return model;
    });
  },
  setupController: function(controller, model) {
    controller.setProperties(model);
  },
  actions: {
    update: function(item) {
      if (!get(item, 'client.blocking')) {
        this.client.abort();
      }
      this.repo.persist(item);
    },
  },
});
