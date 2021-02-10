import RepositoryService from 'consul-ui/services/repository';
import isFolder from 'consul-ui/utils/isFolder';
import { get } from '@ember/object';
import { PRIMARY_KEY } from 'consul-ui/models/kv';
import dataSource from 'consul-ui/decorators/data-source';

const modelName = 'kv';
export default class KvService extends RepositoryService {
  getModelName() {
    return modelName;
  }

  getPrimaryKey() {
    return PRIMARY_KEY;
  }

  // this one gives you the full object so key,values and meta
  @dataSource('/:ns/:dc/kv/*id')
  findBySlug(params, configuration = {}) {
    if (isFolder(params.id)) {
      // TODO: This very much shouldn't be here,
      // needs to eventually use ember-datas generateId thing
      // in the meantime at least our fingerprinter
      const uid = JSON.stringify([params.ns, params.dc, params.id]);
      let item = this.store.peekRecord(this.getModelName(), uid);
      if (!item) {
        item = this.create({
          Key: params.id,
          Datacenter: params.dc,
          Namespace: params.ns,
        });
      }
      return Promise.resolve(item);
    }
    if (typeof configuration.cursor !== 'undefined') {
      params.index = configuration.cursor;
    }
    return this.store.queryRecord(this.getModelName(), params);
  }

  // this one only gives you keys
  // https://www.consul.io/api/kv.html
  findAllBySlug(params, configuration = {}) {
    if (params.id === '/') {
      params.id = '';
    }
    params.separator = '/';
    if (typeof configuration.cursor !== 'undefined') {
      params.index = configuration.cursor;
    }
    return this.store
      .query(this.getModelName(), params)
      .then(function(items) {
        return items.filter(function(item) {
          return params.id !== get(item, 'Key');
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
          const uid = JSON.stringify([params.ns, params.dc, params.id]);
          const record = this.store.peekRecord(this.getModelName(), uid);
          if (record) {
            record.unloadRecord();
          }
        }
        throw e;
      });
  }
}
