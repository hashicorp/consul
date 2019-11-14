import Route from '@ember/routing/route';
import { inject as service } from '@ember/service';
import { hash } from 'rsvp';
import { get, set } from '@ember/object';

export default Route.extend({
  client: service('client/http'),
  repo: service('settings'),
  dcRepo: service('repository/dc'),
  nspaceRepo: service('repository/nspace/disabled'),
  model: function(params) {
    const nspace = this.nspaceRepo.getActive();
    return hash({
      item: this.repo.findAll(),
      dcs: this.dcRepo.findAll(),
      nspaces: this.nspaceRepo.findAll(),
      nspace: this.nspaceRepo.getActive(),
    }).then(model => {
      if (typeof get(model.item, 'client.blocking') === 'undefined') {
        set(model, 'item.client', { blocking: true });
      }
      return hash({
        ...model,
        ...{
          dc: this.dcRepo.getActive(null, model.dcs),
        },
      });
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
