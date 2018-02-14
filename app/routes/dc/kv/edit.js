import Route from '@ember/routing/route';
import { hash } from 'rsvp';
import { get as getter } from '@ember/object';

import Kv from 'consul-ui/models/dc/kv';
import get from 'consul-ui/utils/request/get';
export default Route.extend({
  model: function(params) {
    var key = params.key;
    var dc = this.modelFor('dc').dc;
    var parentKeys = this.getParentAndGrandparent(key);

    // Return a promise hash to get the data for both columns
    return hash({
      dc: dc,
      key: get('/v1/kv/' + key, dc).then(function(data) {
        // Convert the returned data to a Key
        return Kv.create().setProperties(data[0]);
      }),
      keys: get('/v1/kv/' + parentKeys.parent + '?keys&seperator=/', dc).then(function(data) {
        return data.map(function(obj){
          return Kv.create({Key: obj});
        });
      }),
    });
  },

  // Load the session on the key, if there is one
  afterModel: function(models) {
    if (getter(models.key, 'isLocked')) {
      return get('/v1/session/info/' + models.key.Session, models.dc).then(function(data) {
        models.session = data[0];
        return models;
      });
    } else {
      return models;
    }
  },

  setupController: function(controller, models) {
    var key = models.key;
    var parentKeys = this.getParentAndGrandparent(getter(key, 'Key'));
    models.keys = this.removeDuplicateKeys(models.keys, parentKeys.parent);

    controller.set('dc', models.dc);
    controller.set('model', key);
    controller.set('parentKey', parentKeys.parent);
    controller.set('grandParentKey', parentKeys.grandParent);
    controller.set('isRoot', parentKeys.isRoot);
    controller.set('siblings', models.keys);
    controller.set('rootKey', this.rootKey);
    controller.set('session', models.session);
  }
});
