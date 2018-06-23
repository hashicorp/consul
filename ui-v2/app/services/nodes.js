import Service, { inject as service } from '@ember/service';
import { get } from '@ember/object';
export default Service.extend({
  store: service('store'),
  coordinates: service('coordinates'),
  findAllByDatacenter: function(dc) {
    return get(this, 'store').query('node', { dc: dc });
  },
  findBySlug: function(slug, dc) {
    return get(this, 'store')
      .queryRecord('node', {
        id: slug,
        dc: dc,
      })
      .then(node => {
        return get(this, 'coordinates')
          .findAllByDatacenter(dc)
          .then(function(res) {
            node.Coordinates = res;
            return node;
          });
      });
  },
});
