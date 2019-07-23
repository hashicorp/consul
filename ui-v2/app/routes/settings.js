import Route from '@ember/routing/route';
import { inject as service } from '@ember/service';
import { hash } from 'rsvp';
import { get, set } from '@ember/object';

export default Route.extend({
  client: service('client/http'),
  repo: service('settings'),
  dcRepo: service('repository/dc'),
  model: function(params) {
    return hash({
      item: get(this, 'repo').findAll(),
      dcs: get(this, 'dcRepo').findAll(),
    }).then(model => {
      if (typeof get(model.item, 'client.blocking') === 'undefined') {
        set(model, 'item.client', { blocking: true });
      }
      return hash({
        ...model,
        ...{
          dc: get(this, 'dcRepo').getActive(null, model.dcs),
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
        get(this, 'client').abort();
      }
      get(this, 'repo').persist(item);
    },
  },
});
