import Route from '@ember/routing/route';
import { inject as service } from '@ember/service';
import { assign } from '@ember/polyfills';
import { hash } from 'rsvp';
import { get } from '@ember/object';

import WithFeedback from 'consul-ui/mixins/with-feedback';
import ascend from 'consul-ui/utils/ascend';

export default Route.extend(WithFeedback, {
  repo: service('kv'),
  sessionRepo: service('session'),
  model: function(params) {
    const key = params.key;
    const dc = this.modelFor('dc').dc;
    const repo = get(this, 'repo');
    return hash({
      isLoading: false,
      parent: repo.findBySlug(ascend(key, 1) || '/', dc),
      item: repo.findBySlug(key, dc),
    }).then(model => {
      // jc: another afterModel for no reason replacement
      // guessing ember-data will come in here, as we are just stitching stuff together
      const session = get(model.item, 'Session');
      if (session) {
        return hash(
          assign({}, model, {
            session: get(this, 'sessionRepo').findByKey(session, dc),
          })
        );
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
            .persist(item, this.modelFor('dc').dc)
            .then(() => {
              return this.transitionTo('dc.kv.folder', get(parent, 'Key'));
            });
        },
        `Updated ${get(item, 'Key')}`,
        `There was an error updating ${get(item, 'Key')}`
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
        `Deleted ${get(item, 'Key')}`,
        `There was an error deleting ${get(item, 'Key')}`
      );
    },
    // TODO: This is frontend ??
    cancel: function(item, parent) {
      return this.transitionTo('dc.kv.folder', parent);
    },
  },
});
