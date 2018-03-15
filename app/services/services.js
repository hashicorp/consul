import Service, { inject as service } from '@ember/service';

export default Service.extend({
  store: service('store'),
  findAllByDatacenter: function(dc) {
    return this.get('store').query('service', { dc: dc });
  },
  findBySlug: function(slug) {
    return this.get('store')
      .findRecord('service', slug, { reload: true })
      .then(function(service) {
        return service.get('Nodes');
      });
  },
});
