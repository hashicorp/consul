import Service, { inject as service } from '@ember/service';

import get from 'consul-ui/utils/request/get';
import map from 'consul-ui/utils/map';
import Entity from 'consul-ui/models/dc/service';

export default Service.extend({
  // findAllByDatacenter: function(dc) {
  //   return get('/v1/internal/ui/services', dc).then(map(Entity));
  // },
  // findBySlug: function(slug, dc) {
  //   // Here we just use the built-in health endpoint, as it gives us everything
  //   // we need.
  //   return get('/v1/health/service/' + slug, dc).then(map(Entity));
  // },
  store: service('store'),
  findAllByDatacenter: function(datacenter) {
    return this.get('store').query('service', { dc: datacenter });
  },
  findBySlug: function(slug) {
    return this.get('store').findRecord('service', slug);
  },
});
