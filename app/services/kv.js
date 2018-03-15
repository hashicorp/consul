import Service, { inject as service } from '@ember/service';
import { typeOf } from '@ember/utils';

export default Service.extend({
  store: service('store'),
  // this one gives you the full object so key,values and meta
  findBySlug: function(key, dc) {
    return this.get('store').queryRecord('kv', {
      key: key,
      dc: dc,
    });
  },
  // this one only gives you keys
  // https://www.consul.io/api/kv.html
  findAllBySlug: function(key, dc) {
    // TODO: [sic] seperator
    return this.get('store').query('kv', {
      key: key,
      dc: dc,
      seperator: '/',
    });
  },
  create: function() {
    return this.get('store').createRecord('kv');
  },
  persist: function(item, dc) {
    return item.save();
  },
  remove: function(item, dc) {
    if (typeOf(item) === 'object') {
      const key = item.Key;
      item = this.get('store').peekRecord('kv', key);
      if (item == null) {
        item = this.create();
        item.set('Key', key);
      }
    }
    return item.destroyRecord().then(item => {
      // really?
      return this.get('store').unloadRecord(item);
    });
  },
});
