import Service from '@ember/service';
import { Promise } from 'rsvp';
import { get } from '@ember/object';

const SCHEME = 'consul';
const getValue = function(storage, path) {
  let value = storage.getItem(`${SCHEME}:${path}`);
  if (typeof value !== 'string') {
    value = '""';
  }
  try {
    value = JSON.parse(value);
  } catch (e) {
    value = '';
  }
  return value;
};
const setValue = function(storage, path, value) {
  try {
    value = JSON.stringify(value);
  } catch (e) {
    value = '""';
  }
  return storage.setItem(`${SCHEME}:${path}`, value);
};
const removeValue = function(storage, path) {
  return storage.removeItem(`${SCHEME}:${path}`);
};
export default Service.extend({
  // TODO: change name
  storage: window.localStorage,
  findHeaders: function() {
    // TODO: if possible this should be a promise
    // TODO: Actually this has nothing to do with settings it should be in the adapter,
    // which probably can't work with a promise based interface :(
    const token = getValue(get(this, 'storage'), 'token');
    // TODO: The old UI always sent ?token=
    // replicate the old functionality here
    // but remove this to be cleaner if its not necessary
    return {
      'X-Consul-Token': typeof token.SecretID === 'undefined' ? '' : token.SecretID,
    };
  },
  findAll: function(key) {
    const storage = get(this, 'storage');
    const item = Object.keys(storage).reduce((prev, item, i, arr) => {
      if (item.indexOf(`${SCHEME}:`) === 0) {
        prev[item] = getValue(get(this, 'storage'), item);
      }
      return prev;
    }, {});
    return Promise.resolve(item);
  },
  findBySlug: function(slug) {
    return Promise.resolve(getValue(get(this, 'storage'), slug));
  },
  persist: function(obj) {
    const storage = get(this, 'storage');
    Object.keys(obj).forEach((item, i) => {
      setValue(storage, item, obj[item]);
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
      removeValue(storage, item);
      return prev;
    }, {});
    return Promise.resolve(item);
  },
});
