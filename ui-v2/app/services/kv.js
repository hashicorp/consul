import Service, { inject as service } from '@ember/service';
import { typeOf } from '@ember/utils';
import { Promise } from 'rsvp';
import isFolder from 'consul-ui/utils/isFolder';
import { get, set } from '@ember/object';
import { PRIMARY_KEY } from 'consul-ui/models/kv';

export default Service.extend({
  store: service('store'),
  // this one gives you the full object so key,values and meta
  findBySlug: function(key, dc) {
    if (isFolder(key)) {
      const id = JSON.stringify([dc, key]);
      let item = get(this, 'store').peekRecord('kv', id);
      if (!item) {
        item = this.create();
        set(item, 'Key', key);
        set(item, 'Datacenter', dc);
      }
      return Promise.resolve(item);
    }
    return get(this, 'store')
      .queryRecord('kv', {
        id: key,
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
        id: key,
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
      })
      .catch(e => {
        if (e.errors && e.errors[0] && e.errors[0].status == '404') {
          const id = JSON.stringify([dc, key]);
          const record = get(this, 'store').peekRecord('kv', id);
          if (record) {
            record.destroyRecord();
          }
        }
        throw e;
      });
  },
  create: function() {
    return get(this, 'store').createRecord('kv');
  },
  persist: function(item) {
    return item.save();
  },
  remove: function(obj) {
    let item = obj;
    if (typeof obj.destroyRecord === 'undefined') {
      item = obj.get('data');
    }
    if (typeOf(item) === 'object') {
      item = get(this, 'store').peekRecord('kv', item[PRIMARY_KEY]);
    }
    return item.destroyRecord().then(item => {
      return get(this, 'store').unloadRecord(item);
    });
  },
  invalidate: function() {
    return get(this, 'store').unloadAll('kv');
  },
});
