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

    // keys and key are requested in series to avoid
    // ember not being able to merge the responses
    return hash({
      dc: dc,
      // better name, slug vs key?
      keys: repo.findAllBySlug(parentKeys.parent, dc),
    })
      .then(function(model) {
        return hash(
          assign({}, model, {
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
              session: this.get('sessionRepo').findByKey(session, model.dc),
            })
          );
        } else {
          return assign({}, model, {
            session: '',
          });
        }
      })
      .then(model => {
        // TODO: again as in show, look at tidying this up
        const key = model.key;
        const parentKeys = this.getParentAndGrandparent(get(key, 'Key'));
        return assign(model, {
          keys: this.removeDuplicateKeys(model.keys.toArray(), parentKeys.parent),
          model: key,
          parentKey: parentKeys.parent,
          grandParentKey: parentKeys.grandParent,
          isRoot: parentKeys.isRoot,
          siblings: model.keys,
          session: model.session,
          rootKey: this.rootKey,
        });
      });
  },
  setupController: function(controller, model) {
    controller.setProperties(model);
  },
});
