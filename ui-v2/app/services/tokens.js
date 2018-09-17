import Service, { inject as service } from '@ember/service';
import { get } from '@ember/object';
import { typeOf } from '@ember/utils';
import { PRIMARY_KEY } from 'consul-ui/models/token';
export default Service.extend({
  store: service('store'),
  findAllByDatacenter: function(dc) {
    return get(this, 'store').query('token', {
      dc: dc,
    });
  },
  findBySlug: function(slug, dc) {
    return get(this, 'store').queryRecord('token', {
      id: slug,
      dc: dc,
    });
  },
  findByPolicy: function(id, dc) {
    return get(this, 'store').query('token', {
      policy: id,
      dc: dc,
    });
  },
  create: function() {
    return get(this, 'store').createRecord('token');
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
      item = get(this, 'store').peekRecord('token', item[PRIMARY_KEY]);
    }
    return item.destroyRecord().then(item => {
      return get(this, 'store').unloadRecord(item);
    });
  },
  invalidate: function() {
    get(this, 'store').unloadAll('token');
  },
});
