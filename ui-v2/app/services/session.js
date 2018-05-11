import Service, { inject as service } from '@ember/service';
import { get, set } from '@ember/object';

export default Service.extend({
  store: service('store'),
  findByNode: function(node, dc) {
    return get(this, 'store')
      .query('session', {
        id: node,
        dc: dc,
      })
      .then(function(items) {
        return items.map(function(item, i, arr) {
          set(item, 'Datacenter', dc);
          return item;
        });
      });
  },
  findByKey: function(slug, dc) {
    return get(this, 'store')
      .queryRecord('session', {
        id: slug,
        dc: dc,
      })
      .then(function(item) {
        set(item, 'Datacenter', dc);
        return item;
      });
  },
  remove: function(item) {
    return item.destroyRecord().then(item => {
      return get(this, 'store').unloadRecord(item);
    });
  },
});
