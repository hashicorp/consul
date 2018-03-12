import Service, { inject as service } from '@ember/service';

export default Service.extend({
  store: service('store'),
  findByNode: function(node, dc) {
    return this.get('store').query('session', {
      node: node,
      dc: dc,
    });
  },
  findByKey: function(key, dc) {
    return this.get('store').queryRecord('session', {
      key: key,
      dc: dc,
    });
  },
});
