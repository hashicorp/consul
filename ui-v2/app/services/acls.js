import Service, { inject as service } from '@ember/service';
import { get, set } from '@ember/object';
import { typeOf } from '@ember/utils';
import { PRIMARY_KEY } from 'consul-ui/models/acl';
export default Service.extend({
  store: service('store'),
  clone: function(item) {
    return get(this, 'store').clone('acl', get(item, PRIMARY_KEY));
  },
  findAllByDatacenter: function(dc) {
    return get(this, 'store')
      .query('acl', {
        dc: dc,
      })
      .then(function(items) {
        // TODO: Sort with anonymous first
        return items.forEach(function(item, i, arr) {
          set(item, 'Datacenter', dc);
        });
      });
  },
  findBySlug: function(slug, dc) {
    return get(this, 'store')
      .queryRecord('acl', {
        id: slug,
        dc: dc,
      })
      .then(function(item) {
        set(item, 'Datacenter', dc);
        return item;
      });
  },
  create: function() {
    return get(this, 'store').createRecord('acl');
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
      item = get(this, 'store').peekRecord('acl', item[PRIMARY_KEY]);
    }
    return item.destroyRecord().then(item => {
      return get(this, 'store').unloadRecord(item);
    });
  },
  invalidate: function() {
    get(this, 'store').unloadAll('acl');
  },
});
