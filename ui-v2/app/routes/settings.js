import Route from '@ember/routing/route';
import { inject as service } from '@ember/service';
import { hash } from 'rsvp';
import { get } from '@ember/object';

export default Route.extend({
  repo: service('settings'),
  model: function(params) {
    return hash({
      model: get(this, 'repo').findAll(),
    });
  },
  setupController: function(controller, model) {
    controller.setProperties(model);
  },
  actions: {
    update: function(item) {
      get(this, 'feedback').execute(
        () => {
          return get(this, 'repo').persist(item);
        },
        `Your settings were saved.`,
        `There was an error saving your settings.`
      );
    },
    delete: function(key) {
      get(this, 'feedback').execute(
        () => {
          return get(this, 'repo').remove(key);
        },
        `You settings have been reset.`,
        `There was an error resetting your settings.`
      );
    },
  },
});
