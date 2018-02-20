import Route from '@ember/routing/route';
import { inject as service } from '@ember/service';

import { hash } from 'rsvp';
import { get } from '@ember/object';

export default Route.extend({
  repo: service('kv'),
  sessionRepo: service('session'),
  model: function(params) {
    const key = params.key;
    const dc = this.modelFor('dc').dc;
    const parentKeys = this.getParentAndGrandparent(key);
    const repo = this.get('repo');
    // Return a promise hash to get the data for both columns
    return hash({
      dc: dc,
      key: repo.findByKey(key, dc),
      // jc awkward name, see services/kv.js
      keys: repo.findKeysByKey(parentKeys.parent, dc),
    });
  },

  // Load the session on the key, if there is one
  afterModel: function(models) {
    if (get(models.key, 'isLocked')) {
      return this.get('sessionRepo')
        .findByKey(models.key.Session, models.dc)
        .then(function(data) {
          models.session = data[0];
          return models;
        });
    } else {
      return models;
    }
  },

  setupController: function(controller, models) {
    const key = models.key;
    const parentKeys = this.getParentAndGrandparent(get(key, 'Key'));
    models.keys = this.removeDuplicateKeys(models.keys, parentKeys.parent);

    controller.set('dc', models.dc);
    controller.set('model', key);
    controller.set('parentKey', parentKeys.parent);
    controller.set('grandParentKey', parentKeys.grandParent);
    controller.set('isRoot', parentKeys.isRoot);
    controller.set('siblings', models.keys);
    controller.set('rootKey', this.rootKey);
    controller.set('session', models.session);
  },
});
