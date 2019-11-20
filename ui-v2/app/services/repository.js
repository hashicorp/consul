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
    // FIXME: This needs fixing once again to take nspaces into account
    if (typeof meta.date !== 'undefined') {
      this.store.peekAll(this.getModelName()).forEach(item => {
        const date = item.SyncTime;
        if (typeof date !== 'undefined' && date != meta.date) {
          this.store.unloadRecord(item);
        }
      });
    }
  },
  findAllByDatacenter: function(dc, nspace, configuration = {}) {
    const query = {
      dc: dc,
      ns: nspace,
    };
    if (typeof configuration.cursor !== 'undefined') {
      query.index = configuration.cursor;
    }
    return this.store.query(this.getModelName(), query);
  },
  findBySlug: function(slug, dc, nspace, configuration = {}) {
    const query = {
      dc: dc,
      ns: nspace,
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
    // TODO: Change this to use vanilla JS
    // I think this was originally looking for a plain object
    // as opposed to an ember one
    if (typeOf(item) === 'object') {
      item = this.store.peekRecord(this.getModelName(), item[this.getPrimaryKey()]);
    }
    return item.destroyRecord().then(item => {
      return this.store.unloadRecord(item);
    });
  },
  invalidate: function() {
    // TODO: This should probably return a Promise
    this.store.unloadAll(this.getModelName());
  },
});
