import Service, { inject as service } from '@ember/service';
import { get } from '@ember/object';

export default Service.extend({
  store: service('store'),
  findByNode: function(node, dc) {
    return get(this, 'store').query('session', {
      node: node,
      dc: dc,
    });
  },
  findByKey: function(key, dc) {
    return get(this, 'store').queryRecord('session', {
      key: key,
      dc: dc,
    });
  },
  remove: function(item, dc) {
    return item.destroyRecord().then(item => {
      // really?
      return get(this, 'store').unloadRecord(item);
    });
  },
});
