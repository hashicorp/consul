import Service from '@ember/service';
import { Promise } from 'rsvp';
import { get } from '@ember/object';
import getStorage from 'consul-ui/utils/storage/local-storage';
const SCHEME = 'consul';
const storage = getStorage(SCHEME);
export default Service.extend({
  storage: storage,
  findHeaders: function() {
    // TODO: if possible this should be a promise
    // TODO: Actually this has nothing to do with settings it should be in the adapter,
    // which probably can't work with a promise based interface :(
    const token = get(this, 'storage').getValue('token');
    // TODO: The old UI always sent ?token=
    // replicate the old functionality here
    // but remove this to be cleaner if its not necessary
    return {
      'X-Consul-Token': typeof token.SecretID === 'undefined' ? '' : token.SecretID,
    };
  },
  findAll: function(key) {
    return Promise.resolve(get(this, 'storage').all());
  },
  findBySlug: function(slug) {
    return Promise.resolve(get(this, 'storage').getValue(slug));
  },
  persist: function(obj) {
    const storage = get(this, 'storage');
    Object.keys(obj).forEach((item, i) => {
      storage.setValue(item, obj[item]);
    });
    return Promise.resolve(obj);
  },
  delete: function(obj) {
    // TODO: Loop through and delete the specified keys
    if (!Array.isArray(obj)) {
      obj = [obj];
    }
    const storage = get(this, 'storage');
    const item = obj.reduce(function(prev, item, i, arr) {
      storage.removeValue(item);
      return prev;
    }, {});
    return Promise.resolve(item);
  },
});
