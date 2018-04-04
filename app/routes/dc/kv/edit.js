import Route from '@ember/routing/route';
import { inject as service } from '@ember/service';
import { assign } from '@ember/polyfills';

import { hash } from 'rsvp';
import { get } from '@ember/object';
import WithFeedback from 'consul-ui/mixins/with-feedback';
import WithKeyUtils from 'consul-ui/mixins/with-key-utils';
import transitionToNearestParent from 'consul-ui/utils/transitionToNearestParent';
import ascend from 'consul-ui/utils/ascend';

export default Route.extend(WithFeedback, WithKeyUtils, {
  repo: service('kv'),
  sessionRepo: service('session'),
  model: function(params) {
    const key = params.key;
    const parentKey = ascend(key, 1) || '/';
    const dc = this.modelFor('dc').dc;
    const repo = this.get('repo');
    return hash({
      isLoading: false,
      parentKey: parentKey,
      grandParentKey: ascend(key, 2),
      key: repo.findBySlug(key, dc),
    })
      .then(function(model) {
        return hash(
          assign({}, model, {
            siblings: model.keys,
            key: repo.findBySlug(key, dc),
          })
        );
      })
      .then(model => {
        // jc: another afterModel for no reason replacement
        // guessing ember-data will come in here, as we are just stitching stuff together
        const session = model.key.get('Session');
        if (session) {
          return hash(
            assign({}, model, {
              session: this.get('sessionRepo').findByKey(session, dc),
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
    update: function(key) {
      this.get('feedback').execute(
        () => {
          key.set('Value', get(key, 'valueDecoded'));
          return this.get('repo').persist(key, this.modelFor('dc').dc);
        },
        `Updated ${key.get('Key')}`,
        `There was an error updating ${key.get('Key')}`
      );
    },
    delete: function(key) {
      this.get('feedback').execute(
        () => {
          const parentKey = ascend(key.get('Key'), 1) || '/';
          return this.get('repo')
            .remove(key)
            .then(() => {
              const rootKey = this.get('rootKey');
              return transitionToNearestParent.bind(this)(
                this.modelFor('dc').dc,
                parentKey === '/' ? rootKey : parentKey,
                rootKey
              );
            });
        },
        `Deleted ${key.get('Key')}`,
        `There was an error deleting ${key.get('Key')}`
      );
    },
    // TODO: This is frontend ??
    cancel: function(key) {
      const controller = this.controller;
      controller.set('isLoading', true); // check before removing these
      // could probably do with a better notification
      this.transitionTo('dc.kv.show', ascend(key.get('Key'), 1) || '/');
      controller.set('isLoading', false);
    },
  },
});
