import Service from '@ember/service';

import get from 'consul-ui/utils/request/get';
export default Service.extend({
  findAll: function() {
    return get('/v1/catalog/datacenters');
  },
});
