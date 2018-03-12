import Service, { inject as service } from '@ember/service';

import get from 'consul-ui/utils/request/get';
export default Service.extend({
  store: service('store'),
  findByNode: function(node, dc) {
    return this.get('store').query('session', {
      node: node,
      dc: dc,
    });
    // return get('/v1/session/node/' + node, dc);
  },
  findByKey: function(key, dc) {
    return get('/v1/session/info/' + key, dc);
  },
});
