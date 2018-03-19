import Service, { inject as service } from '@ember/service';

export default Service.extend({
  store: service('store'),
  findAllByDatacenter: function(dc) {
    return this.get('store').query('node', { dc: dc });
  },
  findBySlug: function(slug, dc) {
    return this.get('store').queryRecord('node', {
      id: slug,
      dc: dc,
    });
  },
  // findAllCoordinatesByDatacenter: function(dc) {
  //   console.warn('TODO: not ember-data');
  //   return get('/v1/coordinate/nodes', dc);
  // },
});
