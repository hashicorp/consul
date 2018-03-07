import Service, { inject as service } from '@ember/service';

import Entity from 'consul-ui/models/dc/kv';
import get from 'consul-ui/utils/request/get';
export default Service.extend({
  store: service('store'),
  // this one gives you the full object so key,values and meta
  findByKey: function(key, dc) {
    return this.get('store').query('kv', { dc: dc });
  },
  // this one only gives you keys
  // TODO: refactor this into one method with an arg to specify what you want
  // https://www.consul.io/api/kv.html
  findKeysByKey: function(key, dc) {
    // TODO: [sic] seperator
    return this.get('store').query('kv', { dc: dc, keys: '', seperator: '/' });
  },
});
