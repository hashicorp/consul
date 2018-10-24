import Service, { inject as service } from '@ember/service';
import { get } from '@ember/object';
import { typeOf } from '@ember/utils';
import { PRIMARY_KEY, SLUG_KEY } from 'consul-ui/models/token';
import { Promise } from 'rsvp';
import statusFactory from 'consul-ui/utils/acls-status';
const status = statusFactory(Promise);
const MODEL_NAME = 'token';
export default Service.extend({
  getModelName: function() {
    return MODEL_NAME;
  },
  getPrimaryKey: function() {
    return PRIMARY_KEY;
  },
  getSlugKey: function() {
    return SLUG_KEY;
  },
  status: function(obj) {
    return status(obj);
  },
  self: function(secret, dc) {
    return get(this, 'store').self(this.getModelName(), {
      secret: secret,
      dc: dc,
    });
  },
  clone: function(item) {
    return get(this, 'store').clone(this.getModelName(), get(item, PRIMARY_KEY));
  },
  findByPolicy: function(id, dc) {
    return get(this, 'store').query(this.getModelName(), {
      policy: id,
      dc: dc,
    });
  },
  // TODO: RepositoryService
  store: service('store'),
  findAllByDatacenter: function(dc) {
    return get(this, 'store').query(this.getModelName(), {
      dc: dc,
    });
  },
  findBySlug: function(slug, dc) {
    return get(this, 'store').queryRecord(this.getModelName(), {
      id: slug,
      dc: dc,
    });
  },
  create: function(obj) {
    // TODO: This should probably return a Promise
    return get(this, 'store').createRecord(this.getModelName(), obj);
  },
  persist: function(item) {
    return item.save();
  },
  remove: function(obj) {
    let item = obj;
    if (typeof obj.destroyRecord === 'undefined') {
      item = obj.get('data');
    }
    if (typeOf(item) === 'object') {
      item = get(this, 'store').peekRecord(this.getModelName(), item[this.getPrimaryKey()]);
    }
    return item.destroyRecord().then(item => {
      return get(this, 'store').unloadRecord(item);
    });
  },
  invalidate: function() {
    get(this, 'store').unloadAll(this.getModelName());
  },
});
