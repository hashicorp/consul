import Service, { inject as service } from '@ember/service';

import put from 'consul-ui/utils/request/put';
import del from 'consul-ui/utils/request/del';

export default Service.extend({
  store: service('store'),
  // this one gives you the full object so key,values and meta
  findBySlug: function(key, dc) {
    return this.get('store').queryRecord('kv', {
      key: key,
      dc: dc,
    });
  },
  // this one only gives you keys
  // https://www.consul.io/api/kv.html
  findAllBySlug: function(key, dc) {
    // TODO: [sic] seperator
    return this.get('store').query('kv', {
      key: key,
      dc: dc,
      seperator: '/',
    });
  },
  create: function() {
    return this.get('store').createRecord('kv');
  },
  persist: function(key, dc) {
    return put('/v1/kv/' + key.get('Key'), dc, key.get('Value'));
  },
  remove: function(key, dc) {
    return del('/v1/kv/' + key.Key + '?recurse', dc);
  },
});
