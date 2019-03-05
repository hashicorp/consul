import Service, { inject as service } from '@ember/service';
import { get } from '@ember/object';
import { typeOf } from '@ember/utils';
import { PRIMARY_KEY, SLUG_KEY } from 'consul-ui/models/policy';
import { Promise } from 'rsvp';
import statusFactory from 'consul-ui/utils/acls-status';
import isValidServerErrorFactory from 'consul-ui/utils/http/acl/is-valid-server-error';
const isValidServerError = isValidServerErrorFactory();
const status = statusFactory(isValidServerError, Promise);
const MODEL_NAME = 'policy';
export default Service.extend({
  getModelName: function() {
    return MODEL_NAME;
  },
  getPrimaryKey: function() {
    return PRIMARY_KEY;
  },
  getSlugKey: function() {
    return SLUG_KEY;
  },
  store: service('store'),
  status: function(obj) {
    return status(obj);
  },
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
  create: function(obj) {
    return get(this, 'store').createRecord('policy', obj);
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
