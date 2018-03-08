import Route from '@ember/routing/route';
import { inject as service } from '@ember/service';
import { hash } from 'rsvp';
import { assign } from '@ember/polyfills';

export default Route.extend({
  repo: service('kv'),
  model: function(params) {
    let key = params.key || '-';
    // quick hack around not being able to pass an empty
    // string as a wildcard route
    // TODO: this is a breaking change, fix this
    if (key == '-') {
      key = '/';
    }
    const dc = this.modelFor('dc').dc;
    const repo = this.get('repo');
    // Return a promise has with the ?keys for that namespace
    // and the original key requested in params
    return hash({
      dc: dc,
      key: key,
      keys: repo.findKeysByKey(key, dc),
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
        isRoot: parentKeys.isRoot,
      });
    });
  },
  setupController: function(controller, model) {
    controller.setProperties(model);
  },
});
