import Route from '@ember/routing/route';
import { inject as service } from '@ember/service';
import { hash } from 'rsvp';

export default Route.extend({
  repo: service('settings'),
  model: function(params) {
    return hash({
      model: this.get('repo').findAll(),
    });
  },
  actions: {
    update: function(item) {
      this.get('repo').persist(item);
    },
    delete: function(key) {
      this.get('feedback').execute(
        () => {
          return this.get('repo').remove(key);
        },
        `Settings reset`,
        `There was an error resetting your settings`
      );
    },
  },
  setupController: function(controller, model) {
    controller.setProperties(model);
  },
});
