import Route from '@ember/routing/route';
import { inject as service } from '@ember/service';
import { assign } from '@ember/polyfills';

import { hash } from 'rsvp';
import { get } from '@ember/object';
import WithFeedback from 'consul-ui/mixins/with-feedback';
import WithKeyUtils from 'consul-ui/mixins/with-key-utils';
// import transitionToNearestParent from 'consul-ui/utils/transitionToNearestParent';
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
      item: repo.findBySlug(key, dc),
    }).then(model => {
      // jc: another afterModel for no reason replacement
      // guessing ember-data will come in here, as we are just stitching stuff together
      const session = model.item.get('Session');
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
    update: function(item) {
      this.get('feedback').execute(
        () => {
          return get(this, 'repo').persist(item, this.modelFor('dc').dc);
        },
        `Updated ${get(item, 'Key')}`,
        `There was an error updating ${item.get('Key')}`
      );
    },
    delete: function(item) {
      this.get('feedback').execute(
        () => {
          // const parentKey = ascend(get(item, 'Key'), 1) || '/';
          return get(this, 'repo')
            .remove(item)
            .then(() => {
              // const rootKey = this.get('rootKey');
              return this.transitionTo('dc.kv.index');
              // return transitionToNearestParent.bind(this)(
              //   this.modelFor('dc').dc,
              //   parentKey === '/' ? rootKey : parentKey,
              //   rootKey
              // );
            });
        },
        `Deleted ${item.get('Key')}`,
        `There was an error deleting ${item.get('Key')}`
      );
    },
    // TODO: This is frontend ??
    cancel: function(item) {
      const controller = this.controller;
      controller.set('isLoading', true); // check before removing these
      // could probably do with a better notification
      this.transitionTo('dc.kv', ascend(get(item, 'Key'), 1) || '/');
      controller.set('isLoading', false);
    },
  },
});
