import Service from '@ember/service';

export default Service.extend({
  // TODO: change name
  findHeaders: function() {
    return {
      'X-Consul-Token': this.get('token'),
    };
  },
  get: function(key) {
    return window.localStorage.getItem(key);
  },
  set: function(key, value) {
    return window.localStorage.setItem(key, value);
  },
});
