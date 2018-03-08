import Route from '@ember/routing/route';
import { inject as service } from '@ember/service';
import { assign } from '@ember/polyfills';

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
    // the order than keys and key comes in could be important
    // they'll both be arrays of Kv's with the same id's
    return hash({
      dc: dc,
      // jc awkward name, see services/kv.js
      keys: repo.findKeysByKey(parentKeys.parent, dc),
    })
      .then(function(model) {
        return hash(
          assign(model, {
            key: repo.findByKey(key, dc),
          })
        );
      })
      .then(model => {
        // jc: another afterModel for no reason replacement
        // guessing ember-data will come in here, as we are just stitching stuff together
        if (get(model.key, 'isLocked')) {
          return this.get('sessionRepo')
            .findByKey(model.key.Session, model.dc)
            .then(function(data) {
              return assign(model, {
                session: data[0],
              });
            });
        } else {
          return model;
        }
      })
      .then(model => {
        // TODO: again as in show, look at tidying this up
        const key = model.key.get('firstObject');
        const parentKeys = this.getParentAndGrandparent(get(key, 'Key'));
        return assign(model, {
          keys: this.removeDuplicateKeys(model.keys.toArray(), parentKeys.parent),
          model: key,
          parentKey: parentKeys.parent,
          grandParentKey: parentKeys.grandParent,
          isRoot: parentKeys.isRoot,
          siblings: model.keys,
          session: model.session,
        });
      });
  },
  setupController: function(controller, model) {
    controller.setProperties(model);
  },
});
