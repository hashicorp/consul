import Service, { inject as service } from '@ember/service';
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
  reconcile: function(meta = {}) {
    // unload anything older than our current sync date/time
    if (typeof meta.date !== 'undefined') {
      this.store.peekAll(this.getModelName()).forEach(item => {
        const date = item.SyncTime;
        if (typeof date !== 'undefined' && date != meta.date) {
          this.store.unloadRecord(item);
        }
      });
    }
  },
  findAllByDatacenter: function(dc, configuration = {}) {
    const query = {
      dc: dc,
    };
    if (typeof configuration.cursor !== 'undefined') {
      query.index = configuration.cursor;
    }
    return this.store.query(this.getModelName(), query);
  },
  findBySlug: function(slug, dc, configuration = {}) {
    const query = {
      dc: dc,
      id: slug,
    };
    if (typeof configuration.cursor !== 'undefined') {
      query.index = configuration.cursor;
    }
    return this.store.queryRecord(this.getModelName(), query);
  },
  create: function(obj) {
    // TODO: This should probably return a Promise
    return this.store.createRecord(this.getModelName(), obj);
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
      item = this.store.peekRecord(this.getModelName(), item[this.getPrimaryKey()]);
    }
    return item.destroyRecord().then(item => {
      return this.store.unloadRecord(item);
    });
  },
  invalidate: function() {
    this.store.unloadAll(this.getModelName());
  },
});
