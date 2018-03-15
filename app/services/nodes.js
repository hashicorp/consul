import Service, { inject as service } from '@ember/service';

export default Service.extend({
  store: service('store'),
  findAllByDatacenter: function(datacenter) {
    return this.get('store').query('node', { dc: datacenter });
  },
  findBySlug: function(slug) {
    return this.get('store').findRecord('node', slug);
  },
  // findAllCoordinatesByDatacenter: function(dc) {
  //   console.warn('TODO: not ember-data');
  //   return get('/v1/coordinate/nodes', dc);
  // },
});
