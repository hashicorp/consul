import Route from '@ember/routing/route';
import { inject as service } from '@ember/service';
import { hash } from 'rsvp';

export default Route.extend({
  repo: service('kv'),
  model: function(params) {
    let key = params.key || '-';
    // quick hack around not being able to pass an empty
    // string as a wildcard route
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
    });
  },
  setupController: function(controller, models) {
    const key = models.key;
    const repo = this.get('repo');
    const parentKeys = this.getParentAndGrandparent(key);
    models.keys = this.removeDuplicateKeys(models.keys, key);

    controller.set('dc', models.dc);
    controller.set('content', models.keys);
    controller.set('parentKey', parentKeys.parent);
    controller.set('grandParentKey', parentKeys.grandParent);
    controller.set('isRoot', parentKeys.isRoot);
    controller.set('newKey', repo.create());
    controller.set('rootKey', this.rootKey);
  },
});
