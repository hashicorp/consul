import Route from '@ember/routing/route';
import { inject as service } from '@ember/service';
import { hash } from 'rsvp';
import { get } from '@ember/object';

import WithFeedback from 'consul-ui/mixins/with-feedback';
import ascend from 'consul-ui/utils/ascend';

export default Route.extend(WithFeedback, {
  repo: service('kv'),
  sessionRepo: service('session'),
  model: function(params) {
    const key = params.key;
    const dc = this.modelFor('dc').dc.Name;
    const repo = get(this, 'repo');
    return hash({
      isLoading: false,
      parent: repo.findBySlug(ascend(key, 1) || '/', dc),
      item: repo.findBySlug(key, dc),
    }).then(model => {
      // TODO: Consider loading this after initial page load
      const session = get(model.item, 'Session');
      if (session) {
        return hash({
          ...model,
          ...{
            session: get(this, 'sessionRepo').findByKey(session, dc),
          },
        });
      }
      return model;
    });
  },
  setupController: function(controller, model) {
    controller.setProperties(model);
  },
  actions: {
    update: function(item, parent) {
      get(this, 'feedback').execute(
        () => {
          return get(this, 'repo')
            .persist(item)
            .then(() => {
              return this.transitionTo('dc.kv.folder', get(parent, 'Key'));
            });
        },
        `Your key has been saved.`,
        `There was an error saving your key.`
      );
    },
    delete: function(item, parent) {
      get(this, 'feedback').execute(
        () => {
          return get(this, 'repo')
            .remove(item)
            .then(() => {
              return this.transitionTo('dc.kv.folder', get(parent, 'Key'));
            });
        },
        `Your key was deleted.`,
        `There was an error deleting your key.`
      );
    },
    // TODO: This is frontend ??
    cancel: function(item, parent) {
      return this.transitionTo('dc.kv.folder', parent);
    },
  },
});
