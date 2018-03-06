import Service, { inject as service } from '@ember/service';

export default Service.extend({
  store: service('store'),
  findAllByDatacenter: function(datacenter) {
    return this.get('store').query('service', { dc: datacenter });
  },
  findBySlug: function(slug) {
    return this.get('store').findRecord('service', slug, { reload: true });
  },
});
