import Service from '@ember/service';

import get from 'consul-ui/utils/request/get';
import Entity from 'consul-ui/models/dc/acl';
export default Service.extend({
  findByDatacenter: function(dc) {
    return get('/v1/acl/list', dc).then(function(data) {
      const objs = [];
      data.map(function(obj) {
        if (obj.ID === 'anonymous') {
          objs.unshift(Entity.create(obj));
        } else {
          objs.push(Entity.create(obj));
        }
      });
      return objs;
    });
  },
  create: function() {
    return Entity.create();
  },
});
