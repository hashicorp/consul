import Service from '@ember/service';

import get from 'consul-ui/utils/request/get';
export default Service.extend({
  findByNode: function(node, dc) {
    return get('/v1/session/node/' + node, dc);
  },
  findByKey: function(key, dc) {
    return get('/v1/session/info/' + key, dc);
  },
});
