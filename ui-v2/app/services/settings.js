import Service from '@ember/service';
import { Promise } from 'rsvp';
import { get } from '@ember/object';

export default Service.extend({
  // TODO: change name
  storage: window.localStorage,
  findHeaders: function() {
    // TODO: if possible this should be a promise
    return {
      'X-Consul-Token': get(this, 'storage').getItem('token'),
    };
  },
  findAll: function(key) {
    return Promise.resolve({ token: get(this, 'storage').getItem('token') });
  },
  findBySlug: function(slug) {
    return Promise.resolve(get(this, 'storage').getItem(slug));
  },
  persist: function(obj) {
    const storage = get(this, 'storage');
    Object.keys(obj).forEach((item, i) => {
      storage.setItem(item, obj[item]);
    });
    return Promise.resolve(obj);
  },
  delete: function(obj) {
    return Promise.resolve(get(this, 'storage').removeItem('token'));
  },
});
