import Service, { inject as service } from '@ember/service';
import { get } from '@ember/object';
import { assert } from '@ember/debug';
import { typeOf } from '@ember/utils';
export default Service.extend({
  getModelName: function() {
    assert('RepositoryService.getModelName should be overridden', false);
  },
  getPrimaryKey: function() {
    assert('RepositoryService.getPrimaryKey should be overridden', false);
  },
  getSlugKey: function() {
    assert('RepositoryService.getSlugKey should be overridden', false);
  },
  //
  store: service('store'),
  findAllByDatacenter: function(dc) {
    return get(this, 'store').query(this.getModelName(), {
      dc: dc,
    });
  },
  findBySlug: function(slug, dc) {
    return get(this, 'store').queryRecord(this.getModelName(), {
      id: slug,
      dc: dc,
    });
  },
  create: function(obj) {
    // TODO: This should probably return a Promise
    return get(this, 'store').createRecord(this.getModelName(), obj);
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
      item = get(this, 'store').peekRecord(this.getModelName(), item[this.getPrimaryKey()]);
    }
    return item.destroyRecord().then(item => {
      return get(this, 'store').unloadRecord(item);
    });
  },
  invalidate: function() {
    get(this, 'store').unloadAll(this.getModelName());
  },
});
