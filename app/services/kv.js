import Service, { inject as service } from '@ember/service';
import { typeOf } from '@ember/utils';

export default Service.extend({
  store: service('store'),
  // this one gives you the full object so key,values and meta
  findBySlug: function(key, dc) {
    return this.get('store')
      .queryRecord('kv', {
        key: key,
        dc: dc,
      })
      .then(function(item) {
        item.set('Datacenter', dc);
        return item;
      });
  },
  // this one only gives you keys
  // https://www.consul.io/api/kv.html
  findAllBySlug: function(key, dc) {
    // TODO: [sic] seperator
    return this.get('store')
      .query('kv', {
        key: key,
        dc: dc,
        seperator: '/',
      })
      .then(function(items) {
        return items.forEach(function(item, i, arr) {
          item.set('Datacenter', dc);
        });
      });
  },
  create: function() {
    return this.get('store').createRecord('kv');
  },
  persist: function(item) {
    return item.save();
  },
  remove: function(item) {
    if (typeOf(item) === 'object') {
      const key = item.Key;
      // TODO: This won't work for multi dc?
      item = this.get('store').peekRecord('kv', key);
      if (item == null) {
        item = this.create();
        item.set('Key', key);
        item.set('Datacenter', dc);
      }
    }
    return item.destroyRecord().then(item => {
      // really?
      return this.get('store').unloadRecord(item);
    });
  },
});
