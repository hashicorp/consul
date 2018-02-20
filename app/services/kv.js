import Service from '@ember/service';

import Entity from 'consul-ui/models/dc/kv';
import get from 'consul-ui/utils/request/get';
export default Service.extend({
  // jc: this one gives you the full object so key,values and meta
  findByKey: function(key, dc) {
    return get('/v1/kv/' + key, dc).then(function(data) {
      // Convert the returned data to a Key
      return Entity.create().setProperties(data[0]);
    });
  },
  // jc: this one only gives you keys
  // jc: refactor this into one method with an arg to specify what you want
  // https://www.consul.io/api/kv.html
  findKeysByKey: function(key, dc) {
    return get('/v1/kv/' + key + '?keys&seperator=/', dc).then(function(data) {
      return data.map(function(obj) {
        return Entity.create({ Key: obj });
      });
    });
  },
  create: function() {
    return Entity.create();
  },
});
