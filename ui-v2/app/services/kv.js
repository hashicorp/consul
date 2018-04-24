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
    return get(this, 'store')
      .queryRecord('kv', {
        key: key,
        dc: dc,
      })
      .then(function(item) {
        set(item, 'Datacenter', dc);
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
        separator: '/',
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
    return get(this, 'store').createRecord('kv');
  },
  persist: function(item) {
    return item.save();
  },
  remove: function(obj) {
    // TODO: check to see if this is still needed
    // seems like ember-changeset .get('data') still needs this
    //
    let item = obj;
    if (typeof obj.destroyRecord === 'undefined') {
      item = obj.get('data');
    }
    if (typeOf(item) === 'object') {
      const key = item.Key;
      const dc = item.Datacenter;
      // TODO: This won't work for multi dc?
      // id's need to be 'dc-key'
      item = get(this, 'store').peekRecord('kv', key);
      if (item == null) {
        item = this.create();
        set(item, 'Key', key);
        set(item, 'Datacenter', dc);
      }
    }
    return item.destroyRecord().then(item => {
      return get(this, 'store').unloadRecord(item);
    });
  },
});
