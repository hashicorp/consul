import Route from '@ember/routing/route';
import { hash } from 'rsvp';
import Kv from 'consul-ui/models/dc/kv';

import get from 'consul-ui/lib/request/get';
export default Route.extend({
  model: function(params) {
    var key = params.key || "-";
    // quick hack around not being able to pass an empty
    // string as a wildcard route
    if(key == "-") {
      key = "/";
    }
    var dc = this.modelFor('dc').dc;
    // Return a promise has with the ?keys for that namespace
    // and the original key requested in params
    return hash({
      dc: dc,
      key: key,
      keys: get('/v1/kv/' + key + '?keys&seperator=/', dc).then(function(data) {
        return data.map(function(obj) {
          // be careful of this one it's weirder than the other map()'s
          return Kv.create({Key: obj});
        });
      })
    });
  },
  setupController: function(controller, models) {
    var key = models.key;
    var parentKeys = this.getParentAndGrandparent(key);
    models.keys = this.removeDuplicateKeys(models.keys, key);

    controller.set('dc', models.dc);
    controller.set('content', models.keys);
    controller.set('parentKey', parentKeys.parent);
    controller.set('grandParentKey', parentKeys.grandParent);
    controller.set('isRoot', parentKeys.isRoot);
    controller.set('newKey', Kv.create());
    controller.set('rootKey', this.rootKey);
  }
});
