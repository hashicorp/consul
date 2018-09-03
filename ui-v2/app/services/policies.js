import Service, { inject as service } from '@ember/service';
import { get } from '@ember/object';
import { typeOf } from '@ember/utils';
import { PRIMARY_KEY } from 'consul-ui/models/policy';
export default Service.extend({
  store: service('store'),
  translate: function(item) {
    return get(this, 'store').translate('policy', get(item, 'Rules'));
  },
  findAllByDatacenter: function(dc) {
    return get(this, 'store').query('policy', {
      dc: dc,
    });
  },
  findBySlug: function(slug, dc) {
    return get(this, 'store').queryRecord('policy', {
      id: slug,
      dc: dc,
    });
  },
  create: function() {
    return get(this, 'store').createRecord('policy');
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
      item = get(this, 'store').peekRecord('policy', item[PRIMARY_KEY]);
    }
    return item.destroyRecord().then(item => {
      return get(this, 'store').unloadRecord(item);
    });
  },
  invalidate: function() {
    get(this, 'store').unloadAll('policy');
  },
});
