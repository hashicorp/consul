import Service, { inject as service } from '@ember/service';

import get from 'consul-ui/utils/request/get';
import map from 'consul-ui/utils/map';

export default Service.extend({
  store: service('store'),
  findAllByDatacenter: function(datacenter) {
    return this.get('store')
      .findAll('node')
      .then(function(res) {
        return res;
      });
  },
  findBySlug: function(slug) {
    return this.get('store').findRecord('node', slug);
  },
  // findAllByDatacenter: function(dc) {
  //   return get('/v1/internal/ui/nodes', dc).then(map(Entity));
  // },
  findAllCoordinatesByDatacenter: function(dc) {
    return get('/v1/coordinate/nodes', dc);
  },
  // findBySlug: function(slug, dc) {
  //   // maintain consistency with map([])
  //   return get('/v1/internal/ui/node/' + slug, dc).then(map([Entity])[0]);
  // },
});
