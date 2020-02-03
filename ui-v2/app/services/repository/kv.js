import RepositoryService from 'consul-ui/services/repository';
import { Promise } from 'rsvp';
import isFolder from 'consul-ui/utils/isFolder';
import { get } from '@ember/object';
import { PRIMARY_KEY } from 'consul-ui/models/kv';

const modelName = 'kv';
export default RepositoryService.extend({
  getModelName: function() {
    return modelName;
  },
  getPrimaryKey: function() {
    return PRIMARY_KEY;
  },
  // this one gives you the full object so key,values and meta
  findBySlug: function(key, dc, nspace, configuration = {}) {
    if (isFolder(key)) {
      // TODO: This very much shouldn't be here,
      // needs to eventually use ember-datas generateId thing
      // in the meantime at least our fingerprinter
      const id = JSON.stringify([nspace, dc, key]);
      let item = this.store.peekRecord(this.getModelName(), id);
      if (!item) {
        item = this.create({
          Key: key,
          Datacenter: dc,
          Namespace: nspace,
        });
      }
      return Promise.resolve(item);
    }
    const query = {
      id: key,
      dc: dc,
      ns: nspace,
    };
    if (typeof configuration.cursor !== 'undefined') {
      query.index = configuration.cursor;
    }
    return this.store.queryRecord(this.getModelName(), query);
  },
  // this one only gives you keys
  // https://www.consul.io/api/kv.html
  findAllBySlug: function(key, dc, nspace, configuration = {}) {
    if (key === '/') {
      key = '';
    }
    const query = {
      id: key,
      dc: dc,
      ns: nspace,
      separator: '/',
    };
    if (typeof configuration.cursor !== 'undefined') {
      query.index = configuration.cursor;
    }
    return this.store
      .query(this.getModelName(), query)
      .then(function(items) {
        return items.filter(function(item) {
          return key !== get(item, 'Key');
        });
      })
      .catch(e => {
        // TODO: Double check this was loose on purpose, its probably as we were unsure of
        // type of ember-data error.Status at first, we could probably change this
        // to `===` now
        if (get(e, 'errors.firstObject.status') == '404') {
          // TODO: This very much shouldn't be here,
          // needs to eventually use ember-datas generateId thing
          // in the meantime at least our fingerprinter
          const id = JSON.stringify([dc, key]);
          const record = this.store.peekRecord(this.getModelName(), id);
          if (record) {
            record.unloadRecord();
          }
        }
        throw e;
      });
  },
});
