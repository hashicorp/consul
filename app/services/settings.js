import Service from '@ember/service';
import Promise from 'rsvp';

export default Service.extend({
  // TODO: change name
  storage: window.localStorage,
  findHeaders: function() {
    // TODO: if possible this should be a promise
    return {
      'X-Consul-Token': this.get('storage').getItem('token'),
    };
  },
  findAll: function(key) {
    return Promise.resolve({ token: this.get('storage').getItem('token') });
  },
  persist: function(obj) {
    return Promise.resolve(this.get('storage').setItem('token', obj.token));
  },
  delete: function(obj) {
    return Promise.resolve(this.get('storage').removeItem('token'));
  },
});
