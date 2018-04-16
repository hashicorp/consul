import Service, { inject as service } from '@ember/service';
import { typeOf } from '@ember/utils';
import { Promise } from 'rsvp';
import isFolder from 'consul-ui/utils/isFolder';
import { get, set } from '@ember/object';

export default Service.extend({
  store: service('store'),
  // this one gives you the full object so key,values and meta
  findBySlug: function(key, dc) {
    if (isFolder(key)) {
      return Promise.resolve({
        Key: key,
      });
    }
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
    if (key === '/') {
      key = '';
    }
    return this.get('store')
      .query('kv', {
        key: key,
        dc: dc,
        // TODO: [sic] seperator
        seperator: '/',
      })
      .then(function(items) {
        return items
          .filter(function(item) {
            return key !== get(item, 'Key');
          })
          .map(function(item, i, arr) {
            set(item, 'Datacenter', dc);
            return item;
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
      const dc = item.Datacenter;
      // TODO: This won't work for multi dc?
      // id's need to be 'dc-key'
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
