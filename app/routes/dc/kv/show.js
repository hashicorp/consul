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
      key: key,
      keys: repo.findAllBySlug(key, dc),
      newKey: repo.create(),
      isLoading: false,
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
    create: function(key, parentKey, grandParentKey) {
      // If we don't have a previous model to base
      // on our parent, or we're not at the root level,
      // add the prefix
      if (parentKey !== undefined && parentKey !== '/') {
        key.set('Key', parentKey + key.get('Key'));
      }
      this.get('feedback').execute(
        () => {
          return this.get('repo')
            .persist(
              key,
              // TODO: the key object should know its dc, remove this
              this.modelFor('dc').dc
            )
            .then(() => {
              if (key.get('isFolder') === true) {
                this.transitionTo('dc.kv.show', key.get('Key'));
              } else {
                this.transitionTo('dc.kv.edit', key.get('Key'));
              }
            });
        },
        `Created ${key.get('Key')}`,
        `There was an error using ${key.get('Key')}`
      );
    },
    deleteFolder: function(parentKey, grandParent) {
      this.get('feedback').execute(
        () => {
          const dc = this.modelFor('dc').dc;
          // TODO: Possibly change to ember-data entity rather than a pojo
          return this.get('repo')
            .remove(
              {
                Key: parentKey,
              },
              // TODO: the key object should know its dc, remove this
              dc
            )
            .then(response => {
              return transitionToNearestParent.bind(this)(dc, grandParent, this.get('rootKey'));
            });
        },
        `Deleted ${key.get('Key')}`,
        `There was an error deleting ${key.get('Key')}`
      );
    },
  },
});
