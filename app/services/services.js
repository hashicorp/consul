import Service, { inject as service } from '@ember/service';

export default Service.extend({
  store: service('store'),
  findAllByDatacenter: function(dc) {
    return this.get('store')
      .query('service', { dc: dc })
      .then(
        // TODO: Do I actually need to do this?
        function(items) {
          return items.forEach(function(item) {
            item.set('Datacenter', dc);
          });
        }
      );
  },
  findBySlug: function(slug, dc) {
    return this.get('store')
      .queryRecord('service', {
        id: slug,
        dc: dc,
      })
      .then(function(item) {
        return item.get('Nodes');
      });
  },
});
