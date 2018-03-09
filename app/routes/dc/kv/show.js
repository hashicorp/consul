import Route from '@ember/routing/route';
import { inject as service } from '@ember/service';
import { hash } from 'rsvp';
import { assign } from '@ember/polyfills';
import rootKey from 'consul-ui/utils/rootKey';
import transitionToNearestParent from 'consul-ui/utils/transitionToNearestParent';

export default Route.extend({
  repo: service('kv'),
  model: function(params) {
    const key = rootKey(params.key, this.rootKey) || this.rootKey;
    const dc = this.modelFor('dc').dc;
    const repo = this.get('repo');
    // Return a promise has with the ?keys for that namespace
    // and the original key requested in params
    return hash({
      dc: dc,
      key: key,
      keys: repo.findAllBySlug(key, dc),
      rootKey: this.rootKey,
      newKey: repo.create(),
    }).then(model => {
      const key = model.key;
      const parentKeys = this.getParentAndGrandparent(key);
      // TODO: Tidy this up, this is pretty much just a slightly
      // refactored old build
      return assign(model, {
        model: model.keys,
        keys: this.removeDuplicateKeys(model.keys, key),
        parentKey: parentKeys.parent,
        grandParentKey: parentKeys.grandParent,
      });
    });
  },
  setupController: function(controller, model) {
    controller.setProperties(model);
  },
  actions: {
    // Creates the key from the key model argument
    // set on the route.
    createKey: function(key, parentKey, grandParentKey) {
      // If we don't have a previous model to base
      // on our parent, or we're not at the root level,
      // add the prefix
      if (parentKey !== undefined && parentKey !== '/') {
        key.set('Key', parentKey + key.get('Key'));
      }
      const controller = this.controller;
      controller.set('isLoading', true);
      // Put the Key and the Value retrieved from the form
      this.get('repo')
        .persist(
          key,
          // TODO: the key object should know its dc, remove this
          this.modelFor('dc').dc
        )
        .then(response => {
          // transition to the right place
          if (key.get('isFolder') === true) {
            this.transitionTo('dc.kv.show', key.get('Key'));
          } else {
            this.transitionTo('dc.kv.edit', key.get('Key'));
          }
        })
        .catch(function(response) {
          // Render the error message on the form if the request failed
          controller.set('errorMessage', 'Received error while processing: ' + response.statusText);
        })
        .finally(function() {
          controller.set('isLoading', false);
        });
    },
    deleteFolder: function(parentKey, grandParent) {
      const controller = this.controller;
      controller.set('isLoading', true);
      const dc = this.modelFor('dc').dc;
      // TODO: Possibly change to ember-data entity rather than a pojo
      this.get('repo')
        .remove(
          {
            Key: parentKey,
          },
          // TODO: the key object should know its dc, remove this
          dc
        )
        .then(response => {
          return transitionToNearestParent.bind(this)(dc, grandParent, this.get('rootKey'));
        })
        .catch(function(response) {
          // Render the error message on the form if the request failed
          controller.set('errorMessage', 'Received error while processing: ' + response.statusText);
        })
        .finally(function() {
          controller.set('isLoading', false);
        });
    },
  },
});
