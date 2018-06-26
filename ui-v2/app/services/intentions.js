import Service, { inject as service } from '@ember/service';
import { get, set } from '@ember/object';
import { typeOf } from '@ember/utils';
import { PRIMARY_KEY } from 'consul-ui/models/intention';
export default Service.extend({
  store: service('store'),
  findAllByDatacenter: function(dc) {
    return get(this, 'store')
      .query('intention', { dc: dc })
      .then(function(items) {
        return items.forEach(function(item, i, arr) {
          set(item, 'Datacenter', dc);
        });
      });
  },
  findBySlug: function(slug, dc) {
    return get(this, 'store')
      .queryRecord('intention', {
        id: slug,
        dc: dc,
      })
      .then(function(item) {
        set(item, 'Datacenter', dc);
        return item;
      });
  },
  create: function() {
    return get(this, 'store').createRecord('intention');
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
      item = get(this, 'store').peekRecord('intention', item[PRIMARY_KEY]);
    }
    return item.destroyRecord().then(item => {
      return get(this, 'store').unloadRecord(item);
    });
  },
  invalidate: function() {
    return get(this, 'store').unloadAll('intention');
  },
});
