import Service, { inject as service } from '@ember/service';
import { get } from '@ember/object';

export default Service.extend({
  store: service('store'),
  findByNode: function(node, dc) {
    return get(this, 'store').query('session', {
      id: node,
      dc: dc,
    });
  },
  findByKey: function(slug, dc) {
    return get(this, 'store').queryRecord('session', {
      id: slug,
      dc: dc,
    });
  },
  remove: function(item) {
    return item.destroyRecord().then(item => {
      return get(this, 'store').unloadRecord(item);
    });
  },
});
